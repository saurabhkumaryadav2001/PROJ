# Go Backend Daily Learning

Sequential daily curriculum from fundamentals to production backend.

- Day 01: Go foundations, packages, main, variables, types, zero values, constants, fmt
- Day 02: Functions, parameters, return values, scope, errors
- Day 03: Control flow, if/else, for/range, switch, break/continue
- Day 04: Arrays, slices, maps
- Day 05: Structs, methods, pointers ([lesson](day-05/README.md), [code](day-05/structs/main.go), [exercises](day-05/exercises/solutions.go))
- Day 06: Interfaces, implicit satisfaction, interface values, type assertions/switches, typed nil, composition, dependency inversion ([lesson](day-06/README.md), [code](day-06/interfaces/main.go), [exercises](day-06/exercises/solutions.go))
- Day 07: Error handling patterns, sentinel errors, custom errors, wrapping, errors.Is/errors.As, domain-to-HTTP mapping ([lesson](day-07/README.md), [code](day-07/errors/main.go), [exercises](day-07/exercises/solutions.go))
- Day 08: Packages, modules, dependency management ([lesson](day-08/README.md), [API main](day-08/shop/cmd/api/main.go), [catalog](day-08/shop/internal/catalog/catalog.go), [HTTP adapter](day-08/shop/internal/httpapi/handler.go), [exercise](day-08/exercises/demo/main.go))
- Day 09: JSON & File I/O ([lesson](day-09/README.md), [runnable example](day-09/jsonio/main.go), [tests](day-09/jsonio/main_test.go), [sample data](day-09/jsonio/data/products.json))
- Day 10: HTTP server and REST APIs ([lesson](day-10/README.md), [runnable REST API](day-10/restapi/main.go), [tests](day-10/restapi/main_test.go))
- Day 11: Middleware, routing, request validation ([lesson](day-11/README.md), [server](day-11/middlewareapi/main.go), [middleware](day-11/middlewareapi/middleware.go), [handlers](day-11/middlewareapi/handlers.go), [tests](day-11/middlewareapi/main_test.go))
- **Day 12: PostgreSQL, database/sql, connection pooling and SQL transactions** ([lesson](day-12/README.md), [API](day-12/postgresapi/main.go), [database pool](day-12/postgresapi/db.go), [driver registration](day-12/postgresapi/driver_postgres.go), [repository and transaction](day-12/postgresapi/repository.go), [handlers](day-12/postgresapi/handlers.go), [migration](day-12/postgresapi/migrations/001_create_products_and_orders.sql), [tests](day-12/postgresapi/main_test.go))
- Day 13: Context, cancellation, timeouts
- Day 14: Goroutines, channels, synchronization
- Day 15+: testing, Docker, config, logging, auth, observability, CI/CD, project build

Complete each day's exercises before moving to the next.

## Day 12 quick start

The PostgreSQL driver is pinned in go.mod and registered through the `postgres` build tag. Download it, apply the migration, set DATABASE_URL, then run:

```bash
go mod download github.com/jackc/pgx/v5
psql "$DATABASE_URL" -f day-12/postgresapi/migrations/001_create_products_and_orders.sql
go run -tags postgres ./day-12/postgresapi
```

Handler tests run with a fake repository and do not require a live database:

```bash
go test ./day-12/postgresapi
go test -race ./day-12/postgresapi
go vet ./day-12/postgresapi
```
