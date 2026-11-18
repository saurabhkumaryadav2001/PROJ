# Day 12 — PostgreSQL, database/sql, Connection Pooling & Transactions

**Previous:** Day 11 — Middleware, Routing & Request Validation  
**Next:** Day 13 — Context, Cancellation & Timeouts

## Learning goals

- Understand the relationship between Go's database/sql package and a database driver.
- Open and tune a PostgreSQL connection pool.
- Run parameterized INSERT, SELECT and multi-row queries.
- Scan rows, close result sets and check rows.Err().
- Map sql.ErrNoRows to a domain-level not-found error.
- Use BeginTx, Commit and Rollback.
- Prevent inventory overselling with a conditional UPDATE inside a transaction.
- Test HTTP handlers with a fake repository and run an optional PostgreSQL integration test.

## Analogy: a bank transfer

A bank transfer must debit one account and credit another together. If the credit fails, the debit must be rolled back. A transaction makes both changes one unit: both commit or neither does.

Our order operation does the same:
1. Decrease product stock by the requested quantity.
2. Insert the order row.
3. Commit only after both steps succeed.

If order insertion fails, rollback restores the stock.

## database/sql and the PostgreSQL driver

database/sql is Go's standard SQL API. It provides a common interface for connection pools, queries, rows and transactions. It does not speak PostgreSQL's wire protocol itself. This project uses github.com/jackc/pgx/v5/stdlib, which registers the driver name "pgx".

The driver import is isolated in driver_postgres.go behind the postgres build tag. This permits offline handler tests without downloading the external driver. The real PostgreSQL application must be built with -tags postgres.

## Project structure

```text
go-backend-daily/
├── go.mod
└── day-12/
    ├── README.md
    └── postgresapi/
        ├── main.go
        ├── db.go
        ├── driver_postgres.go
        ├── repository.go
        ├── handlers.go
        ├── main_test.go
        └── migrations/001_create_products_and_orders.sql
```

## Database schema

The migration creates products and orders. Product prices are stored in paise as BIGINT, not float64. ₹1499.00 is stored as 149900 paise. Integer minor units avoid floating-point rounding for exact currency arithmetic.

Constraints:
- SKU is unique.
- Price must be positive.
- Stock cannot be negative.
- Order quantity and amounts must be positive.
- orders.product_id references products(id).
- An order stores unit price and total at purchase time, so later product price changes do not rewrite history.

Apply the migration to a local or disposable PostgreSQL database:

```bash
export DATABASE_URL='postgres://postgres:postgres@localhost:5432/go_backend?sslmode=disable'
psql "$DATABASE_URL" -f day-12/postgresapi/migrations/001_create_products_and_orders.sql
```

PowerShell:

```powershell
$env:DATABASE_URL = "postgres://postgres:postgres@localhost:5432/go_backend?sslmode=disable"
psql $env:DATABASE_URL -f day-12/postgresapi/migrations/001_create_products_and_orders.sql
```

Download the pinned driver and run the actual PostgreSQL-backed server:

```bash
go mod download github.com/jackc/pgx/v5
go run -tags postgres ./day-12/postgresapi
```

The server requires DATABASE_URL, pings the database at startup and listens on port 8080 by default. Set PORT to change it.

## Connection pooling

```go
db.SetMaxOpenConns(20)
db.SetMaxIdleConns(5)
db.SetConnMaxIdleTime(5 * time.Minute)
db.SetConnMaxLifetime(30 * time.Minute)
```

- MaxOpenConns caps simultaneous open connections.
- MaxIdleConns controls reusable idle connections.
- ConnMaxIdleTime retires connections that have been idle too long.
- ConnMaxLifetime periodically recycles long-lived connections.

sql.Open creates a pool manager; PingContext verifies connectivity. Pool limits multiply across replicas: 20 connections × 10 replicas can mean 200 database connections. Tune against PostgreSQL's connection budget and workload.

## API endpoints

| Method | Path | Purpose |
|---|---|---|
| GET | /healthz | Check database connectivity |
| GET | /api/v1/products?limit=20&offset=0 | List products |
| POST | /api/v1/products | Create product |
| GET | /api/v1/products/{id} | Get product |
| POST | /api/v1/orders | Reserve stock and create order transactionally |

Create product (₹1499.00 = 149900 paise):

```bash
curl -i -X POST http://localhost:8080/api/v1/products \
  -H 'Content-Type: application/json' \
  -d '{"sku":"KB-01","name":"Keyboard","price_paise":149900,"stock":10}'
```

Create order:

```bash
curl -i -X POST http://localhost:8080/api/v1/orders \
  -H 'Content-Type: application/json' \
  -d '{"product_id":1,"quantity":2}'
```

A missing product returns 404. Insufficient stock returns 409. Successful creation returns 201.

## Code walkthrough

### db.go
- sql.Open("pgx", databaseURL) creates the pool manager.
- Pool settings cap concurrency and recycle connections.
- PingContext verifies the server is reachable within the startup deadline.
- On failure the pool is closed; errors are wrapped with %w.

### driver_postgres.go
- The postgres build tag includes this file only when requested.
- The blank pgx stdlib import registers driver name pgx.
- Fake-repository tests compile without the tag; the real app needs -tags postgres.

### repository.go
- ErrNotFound and ErrInsufficientStock are domain sentinel errors.
- Repository is the interface consumed by handlers; SQLRepository implements it.
- QueryRowContext runs a one-row query; Scan copies columns into Go fields.
- sql.ErrNoRows maps to ErrNotFound.
- QueryContext returns rows; always close them and check rows.Err().
- Every user value is passed using PostgreSQL placeholders ($1, $2, ...).
- CreateOrder starts a transaction, conditionally decrements stock, inserts an order and commits.
- defer tx.Rollback() protects early returns; after Commit it is harmless.
- All statements in the transaction use tx, never db.
- Integer paise avoids floating-point currency errors; multiplication overflow is checked.

### handlers.go
- API depends on Repository rather than *sql.DB.
- Method-aware Go 1.22+ routes map HTTP methods and paths to handlers.
- decodeJSON checks application/json, caps the body at 1 MiB, rejects unknown fields and rejects trailing JSON values.
- Product/order input is validated before repository calls.
- Pagination is bounded to avoid unbounded queries.
- ErrNotFound maps to 404; ErrInsufficientStock maps to 409.
- Internal SQL details are not exposed to API clients.

### main.go
- Requires DATABASE_URL.
- Opens and pings the pool with a five-second startup timeout.
- Wires SQLRepository into API.
- Configures HTTP read, write and idle timeouts.
- Handles SIGINT/SIGTERM and gracefully shuts down in-flight requests.
- Closes the database pool on process exit.

### main_test.go
- fakeRepository implements Repository without PostgreSQL.
- Handler tests cover create/list, malformed JSON, unknown fields, validation, wrong content type, oversized body, order success, insufficient stock, missing product and health.
- Optional TestPostgresOrderDoesNotOversell uses a disposable PostgreSQL database and launches 20 concurrent order attempts against stock of 5. Exactly five should succeed.

## The transaction in detail

```go
tx, err := db.BeginTx(ctx, &sql.TxOptions{Isolation: sql.LevelReadCommitted})
if err != nil { return err }
defer tx.Rollback()

// Use tx.QueryContext / tx.QueryRowContext / tx.ExecContext
// for every statement in this logical operation.

if err := tx.Commit(); err != nil { return err }
```

- BEGIN starts the unit of work.
- COMMIT makes all changes succeed together.
- ROLLBACK discards uncommitted changes.
- Do not call db.ExecContext in the middle; it can use another connection outside the transaction.
- Keep transactions short. Do not call slow external APIs while holding database locks.
- Transactions alone do not prevent every race. Use constraints and conditional updates to protect business invariants.

The critical stock statement is:

```sql
UPDATE products
SET stock = stock - $1
WHERE id = $2 AND stock >= $1
RETURNING price_paise;
```

The stock predicate and decrement are one atomic SQL statement. PostgreSQL serializes conflicting row updates, preventing the stock from becoming negative through this operation. The order insert follows in the same transaction. If it fails, rollback undoes the decrement.

## Run tests

Handler tests do not require PostgreSQL:

```bash
go test ./day-12/postgresapi
go test -race ./day-12/postgresapi
go vet ./day-12/postgresapi
go build ./day-12/postgresapi
```

Optional integration test requires a disposable database with migration applied. It truncates products and orders; never point it at production:

```bash
export TEST_DATABASE_URL="$DATABASE_URL"
go test -tags postgres ./day-12/postgresapi -run TestPostgresOrderDoesNotOversell -count=1
```

## Exercises and suggested solutions

1. **Add GET /api/v1/orders/{id}.** Add GetOrder(ctx,id) to the interface and SQL repository; map sql.ErrNoRows to ErrNotFound and HTTP 404.
2. **Add DELETE /api/v1/products/{id}.** Use DELETE and RowsAffected. The foreign key blocks deletion when orders exist; map the constraint to conflict or implement soft delete.
3. **Insert an audit row with each order.** Begin one transaction, reserve stock, insert order, insert audit row, commit once. All statements must use tx.
4. **Map duplicate SKU to 409.** Inspect pgx PostgreSQL error code 23505 inside the repository adapter and return a domain ErrConflict.
5. **Prove rollback when order insertion fails.** In a disposable database, add a test-only trigger that raises on orders insert; verify stock remains unchanged.
6. **Tune pool size for three replicas.** Multiply MaxOpenConns by three and stay below PostgreSQL's connection budget.

## Interview questions

1. What is database/sql? — Go's standard SQL API; a driver implements the database protocol.
2. Does sql.Open prove connectivity? — No; call PingContext.
3. What is *sql.DB? — A concurrency-safe connection-pool manager.
4. QueryRow vs Query? — One row vs a row iterator; Query rows must be closed and checked.
5. What is parameterized SQL? — Placeholders plus separate values, preventing SQL injection.
6. What does sql.ErrNoRows mean? — No matching row; often a normal not-found case.
7. What is a transaction? — Statements committed or rolled back as one unit.
8. What is ACID? — Atomicity, Consistency, Isolation and Durability.
9. Why defer tx.Rollback? — It protects early returns; after Commit the transaction is already complete.
10. Why use tx for every statement? — db may choose another pooled connection outside the transaction.
11. How does conditional stock UPDATE prevent overselling? — The stock predicate and row update are evaluated atomically while conflicting updates serialize.
12. Why store currency in paise? — Integer minor units avoid float rounding.
13. What is pool exhaustion? — All allowed connections are busy; callers wait or hit their context deadline.
14. Should a transaction call an external API? — Usually not; keep locks short and use an outbox/saga for cross-system workflows.

## Production checklist

- [ ] Keep database credentials in a secret manager; never commit them.
- [ ] Use TLS outside local development.
- [ ] Count pool limits across all replicas.
- [ ] Propagate request contexts and query deadlines.
- [ ] Parameterize every user-controlled SQL value.
- [ ] Close rows and check rows.Err().
- [ ] Keep transactions short and use tx for every statement in the unit.
- [ ] Add database constraints for critical invariants.
- [ ] Map expected database errors to domain errors; do not expose SQL text to clients.
- [ ] Apply migrations through a controlled migration process.
- [ ] Back up and monitor PostgreSQL.
- [ ] Test concurrency-sensitive behavior against real disposable PostgreSQL.

## Next lesson

**Day 13 — Context, Cancellation & Timeouts:** propagate HTTP request context into SQL calls, cancel work when a client disconnects, set query deadlines and prevent goroutine leaks.
