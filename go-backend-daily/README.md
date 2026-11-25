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
- Day 12: PostgreSQL, database/sql, connection pooling and SQL transactions ([lesson](day-12/README.md), [API](day-12/postgresapi/main.go), [database pool](day-12/postgresapi/db.go), [driver registration](day-12/postgresapi/driver_postgres.go), [repository and transaction](day-12/postgresapi/repository.go), [migration](day-12/postgresapi/migrations/001_create_products_and_orders.sql), [tests](day-12/postgresapi/main_test.go))
- Day 13: Context, cancellation, timeouts, request propagation, bounded cancellable workers and graceful shutdown ([lesson](day-13/README.md), [server](day-13/contextapi/main.go), [middleware](day-13/contextapi/context.go), [handlers](day-13/contextapi/handlers.go), [worker pool](day-13/contextapi/work.go), [tests](day-13/contextapi/main_test.go))
- **Day 14: Goroutines, channels, select, WaitGroup, Mutex/RWMutex, fan-out/fan-in, backpressure and bounded worker pools** ([lesson](day-14/README.md), [server](day-14/concurrencyapi/main.go), [worker pool](day-14/concurrencyapi/pool.go), [race-safe metrics](day-14/concurrencyapi/metrics.go), [HTTP handlers](day-14/concurrencyapi/handlers.go), [tests](day-14/concurrencyapi/main_test.go))
- **Day 15: Testing Go backends — table-driven tests, subtests, httptest, fakes, PostgreSQL integration tests, coverage, fuzzing and benchmarks** ([lesson](day-15/README.md), [API](day-15/testingapi/main.go), [service](day-15/testingapi/product.go), [repositories](day-15/testingapi/repository.go), [HTTP handlers](day-15/testingapi/handler.go), [service tests](day-15/testingapi/service_test.go), [HTTP tests](day-15/testingapi/handler_test.go), [PostgreSQL integration test](day-15/testingapi/postgres_integration_test.go))
- **Day 16: Docker, multi-stage builds, typed environment configuration, structured JSON logs, health/readiness probes, container security, Compose, networking and graceful shutdown** ([lesson](day-16/README.md), [Dockerfile](day-16/Dockerfile), [Compose](day-16/compose.yaml), [configuration](day-16/runtimeapi/config.go), [server](day-16/runtimeapi/main.go), [handlers](day-16/runtimeapi/handlers.go), [tests](day-16/runtimeapi/config_test.go))
- **Day 17: Observability — structured JSON logs, Prometheus counters/gauges/histograms, bounded route labels, W3C Trace Context propagation, spans, Prometheus/Grafana and PromQL** ([lesson](day-17/README.md), [API](day-17/observabilityapi/main.go), [metrics](day-17/observabilityapi/metrics.go), [tracing](day-17/observabilityapi/trace.go), [middleware](day-17/observabilityapi/middleware.go), [tests](day-17/observabilityapi/observability_test.go), [Compose](day-17/compose.yaml))
- **Day 18: Authentication & Authorization — PBKDF2 password hashing, HS256 JWT access tokens, opaque refresh-token rotation, Bearer middleware and RBAC** ([lesson](day-18/README.md), [auth core](day-18/authapi/core.go), [HTTP API](day-18/authapi/api.go), [tests](day-18/authapi/auth_test.go), [PostgreSQL schema](day-18/migrations/001_auth_tables.sql))
- **Day 19: PostgreSQL-backed authentication — `database/sql`, pgx, connection pooling, parameterized SQL, unique-email constraints, transactions, `SELECT FOR UPDATE`, refresh-token rotation and replay detection** ([lesson](day-19/README.md), [PostgreSQL repository](day-19/authapi/postgres_store.go), [service](day-19/authapi/service.go), [handlers](day-19/authapi/handlers.go), [migration](day-19/migrations/001_auth_tables.sql), [tests](day-19/authapi/service_test.go))

## Day 17 quick start

The API uses only the Go standard library:

~~~bash
go test ./day-17/observabilityapi
go test -race ./day-17/observabilityapi
go vet ./day-17/observabilityapi
go test -cover ./day-17/observabilityapi
go run ./day-17/observabilityapi
~~~

Run the local Prometheus/Grafana stack (set GRAFANA_ADMIN_PASSWORD first):

~~~bash
docker compose -f day-17/compose.yaml up --build -d
docker compose -f day-17/compose.yaml logs -f
docker compose -f day-17/compose.yaml down
~~~

Complete each day's exercises before moving to the next.

## Day 12 quick start

The PostgreSQL driver is pinned in go.mod and registered through the postgres build tag. Download it, apply the migration, set DATABASE_URL, then run:

~~~bash
go mod download github.com/jackc/pgx/v5
psql "$DATABASE_URL" -f day-12/postgresapi/migrations/001_create_products_and_orders.sql
go run -tags postgres ./day-12/postgresapi
~~~

Handler tests run with a fake repository and do not require a live database:

~~~bash
go test ./day-12/postgresapi
go test -race ./day-12/postgresapi
go vet ./day-12/postgresapi
~~~

## Day 13 quick start

Day 13 uses only the standard library:

~~~bash
go run ./day-13/contextapi
go test ./day-13/contextapi
go test -race ./day-13/contextapi
go vet ./day-13/contextapi
~~~

## Day 14 quick start

Day 14 uses only the standard library. Configure the worker count and bounded queue, then run:

~~~bash
WORKERS=4 QUEUE_SIZE=8 go run ./day-14/concurrencyapi
go test ./day-14/concurrencyapi
go test -race ./day-14/concurrencyapi
go vet ./day-14/concurrencyapi
go build -o ./bin/concurrency-api ./day-14/concurrencyapi
~~~

PowerShell:

~~~powershell
$env:WORKERS = "4"
$env:QUEUE_SIZE = "8"
go run ./day-14/concurrencyapi
~~~


## Day 15 quick start

The runnable API uses an in-memory repository and the standard library:

~~~bash
go run ./day-15/testingapi
go test ./day-15/testingapi
go test -race ./day-15/testingapi
go vet ./day-15/testingapi
go test -cover ./day-15/testingapi
go test -bench=BenchmarkProductServiceCreate -benchmem ./day-15/testingapi
go test -fuzz=FuzzParseProductID -fuzztime=10s ./day-15/testingapi
~~~

The optional PostgreSQL integration test uses the existing pgx module dependency and is excluded from the default test run:

~~~bash
go mod download github.com/jackc/pgx/v5
TEST_DATABASE_URL="postgres://user:password@localhost:5432/testdb?sslmode=disable" go test -tags="integration pgx" ./day-15/testingapi -run TestPostgresRepositoryIntegration -v
~~~


## Day 16 quick start

Day 16 uses the Go standard library for the application. Run locally:

~~~bash
go test ./day-16/runtimeapi
go test -race ./day-16/runtimeapi
go vet ./day-16/runtimeapi
go run ./day-16/runtimeapi
~~~

Build and run the multi-stage container from the module root:

~~~bash
docker build -f day-16/Dockerfile -t go-runtime-api:day16 .
docker run --rm -p 8080:8080 go-runtime-api:day16
~~~

Or use Compose:

~~~bash
docker compose --env-file day-16/.env.example -f day-16/compose.yaml up --build -d
docker compose --env-file day-16/.env.example -f day-16/compose.yaml logs -f api
docker compose --env-file day-16/.env.example -f day-16/compose.yaml down
~~~


## Day 18 quick start

Day 18 reuses the root Go module and uses the standard library:

~~~bash
gofmt -w day-18/authapi/*.go
go test ./day-18/authapi
go test -race ./day-18/authapi
go vet ./day-18/authapi
go test -cover ./day-18/authapi
go run ./day-18/authapi
~~~

The app exposes registration, login, refresh-token rotation, GET /api/me and an admin-only GET /api/admin/users endpoint. See [Day 18 README](day-18/README.md) for curl examples, security boundaries and exercises.


## Day 19 quick start

Day 19 uses the existing root module and the pgx v5.7.2 dependency. Apply the migration, set DATABASE_URL and a unique JWT_SECRET, then:

~~~bash
go test ./day-19/authapi
go test -race ./day-19/authapi
go vet ./day-19/authapi
go run -tags pgx ./day-19/authapi
~~~

Apply the schema with:

~~~bash
psql "$DATABASE_URL" -f day-19/migrations/001_auth_tables.sql
~~~

Download the pgx driver if needed with `go mod download github.com/jackc/pgx/v5`. The real PostgreSQL integration test uses build tags `pgx integration` and TEST_DATABASE_URL.
