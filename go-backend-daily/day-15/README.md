# Day 15 — Testing Go Backend Applications

**Previous:** Day 14 — Goroutines, channels, synchronization and worker pools  
**Next:** Day 16 — Docker, configuration and production runtime foundations

Day 15 teaches how to prove backend behavior with automated tests. The sample is a small product REST API with a service/repository boundary, a runnable in-memory adapter, a PostgreSQL adapter and several layers of tests.

## Learning goals

- Distinguish unit, handler/component, integration and end-to-end tests.
- Write table-driven tests and named subtests with `t.Run`.
- Use `t.Helper`, `t.Cleanup` and `t.Parallel` safely.
- Test HTTP handlers with `net/http/httptest`.
- Replace repositories with fakes and verify errors with `errors.Is`.
- Test status codes, headers, JSON, validation and body limits.
- Run coverage, race detection, fuzz tests and benchmarks.
- Understand optional real-PostgreSQL integration tests.

## 1. Why testing matters

A backend can compile and still be wrong. A small change to validation may accidentally allow a zero price; a handler may return 200 instead of 201; a database error may leak internal details. Tests make expected behavior executable and catch regressions before deployment.

Analogy: a building inspector checks the foundation, wiring, plumbing and whole building. Unit tests check small components; handler tests check the HTTP boundary; integration tests check real dependencies; end-to-end tests check a complete user journey.

## 2. Testing levels

| Level | What it checks | Typical approach |
|---|---|---|
| Unit | One function/type with dependencies replaced | `testing` package + fake |
| Handler/component | Router + handler + service + fake repository | `httptest.NewRecorder` |
| Integration | Real PostgreSQL/cache/broker | Real dependency + isolated fixture |
| End-to-end | Complete deployed user workflow | External client/browser/test runner |

Prefer many fast unit/handler tests, fewer integration tests and a small number of end-to-end tests.

## 3. Project structure

```text
go-backend-daily/
  go.mod
  day-15/
    README.md
    testingapi/
      main.go
      product.go
      repository.go
      handler.go
      service_test.go
      handler_test.go
      postgres_integration_test.go
      product_fuzz_test.go
      benchmark_test.go
```

The default app and tests use the Go standard library. The optional PostgreSQL test uses pgx v5, already listed in the parent module's go.mod.

## 4. Run the project

Run from the `go-backend-daily` module root:

```bash
gofmt -w day-15/testingapi/*.go
go run ./day-15/testingapi
go test ./day-15/testingapi
go test -race ./day-15/testingapi
go vet ./day-15/testingapi
go test -cover ./day-15/testingapi
go test -coverprofile=coverage.out ./day-15/testingapi
go tool cover -func=coverage.out
go test -bench=BenchmarkProductServiceCreate -benchmem ./day-15/testingapi
go test -fuzz=FuzzParseProductID -fuzztime=10s ./day-15/testingapi
```

PowerShell:

```powershell
$env:PORT = "8090"
go run ./day-15/testingapi
go test ./day-15/testingapi
go test -race ./day-15/testingapi
go test -cover ./day-15/testingapi
```

## 5. Application architecture

- `product.go`: domain models, repository interface, business validation and service.
- `repository.go`: thread-safe in-memory adapter and SQL adapter.
- `handler.go`: REST routes, strict JSON decoding, body-size protection and HTTP error mapping.
- `main.go`: application wiring and graceful server shutdown.
- Test files: isolated service tests, HTTP contract tests, optional PostgreSQL integration test, fuzz target and benchmark.

The key test seam is `ProductRepository`. ProductService depends on the interface, not PostgreSQL. A fake can force success, not-found and database failures without opening a database.

## 6. Table-driven tests and subtests

A table-driven test stores named inputs and expected outputs in a slice, then loops through them:

```go
cases := []struct {
    name string
    price int64
    wantErr bool
}{
    {name: "positive", price: 100, wantErr: false},
    {name: "zero", price: 0, wantErr: true},
    {name: "negative", price: -1, wantErr: true},
}
for _, tc := range cases {
    t.Run(tc.name, func(t *testing.T) {
        err := validatePrice(tc.price)
        if (err != nil) != tc.wantErr {
            t.Fatalf("error = %v, wantErr = %v", err, tc.wantErr)
        }
    })
}
```

Why it matters: cases are easy to add, each failure has a useful name, and the same assertion logic is reused. `t.Run` creates a named subtest; use `go test -run 'TestName/case_name'` to focus on one case.

## 7. Fakes, stubs and mocks

A fake is a small test implementation of a dependency. A stub returns predetermined values. A mock additionally records/verifies interactions.

The `fakeRepository` in `service_test.go` has function fields so a test can make Create return a chosen database error. The service wraps it using `fmt.Errorf("...: %w", err)`; the test uses `errors.Is` to prove the original error identity is preserved.

Use fakes at meaningful boundaries (repository, payment gateway, email sender), not for every tiny function.

## 8. HTTP tests with httptest

`httptest.NewRecorder` is an in-memory ResponseWriter. `httptest.NewRequest` builds an incoming request. Calling `handler.ServeHTTP(rec, req)` tests the handler without opening a TCP port:

```go
req := httptest.NewRequest(http.MethodGet, "/healthz", nil)
rec := httptest.NewRecorder()
handler.ServeHTTP(rec, req)
if rec.Code != http.StatusOK {
    t.Fatalf("status = %d, want 200", rec.Code)
}
```

Use `httptest.NewServer` when the code under test must behave as an HTTP client. It starts a temporary local server and gives you a URL. Always close it; the sample uses `t.Cleanup(server.Close)`.

The HTTP tests verify:
- POST success, 201, Location header and JSON body.
- Malformed JSON, unknown fields and multiple JSON documents.
- Invalid domain data (422) and wrong content type (415).
- GET success, missing ID (404), invalid ID (400).
- Pagination and deterministic result order.
- Oversized request body (413).
- Health and method-not-allowed (405).
- Internal error responses do not reveal fake secrets or database hostnames.

## 9. Important testing helpers

- `t.Helper()`: marks a helper so failure locations point to the calling test.
- `t.Cleanup(fn)`: runs cleanup after the test and its subtests finish.
- `t.Parallel()`: runs a test/subtest concurrently. Only use it when tests do not share mutable fixtures or shared external state.
- `t.Fatal`: stops the calling test goroutine. Do not call it from a goroutine created by the test; send errors to the test goroutine or use `t.Errorf` with synchronization.
- Close response bodies and test servers.
- Avoid `time.Sleep` for synchronization; use channels, contexts and explicit completion signals.

## 10. Integration testing with PostgreSQL

Unit tests with a fake cannot prove SQL syntax, schema names, driver behavior or database constraints. The optional `postgres_integration_test.go` uses a real PostgreSQL connection through pgx and creates a temporary table named products.

The test obtains one dedicated `*sql.Conn`, creates the temporary table on that connection, then passes that same connection to SQLRepository. PostgreSQL temporary tables are session-scoped, so using the same connection matters. Closing it drops the table. The test inserts, reads, checks not-found mapping and lists a product.

The file has build constraints `integration && pgx`, so normal `go test` remains fast and does not require a database driver download. The parent module already lists pgx v5.7.2. On a network-enabled machine, download the module if needed, set a disposable test DSN and run:

```bash
go mod download github.com/jackc/pgx/v5
TEST_DATABASE_URL="postgres://user:password@localhost:5432/testdb?sslmode=disable" go test -tags="integration pgx" ./day-15/testingapi -run TestPostgresRepositoryIntegration -v
```

PowerShell:

```powershell
$env:TEST_DATABASE_URL = "postgres://user:password@localhost:5432/testdb?sslmode=disable"
go test -tags="integration pgx" ./day-15/testingapi -run TestPostgresRepositoryIntegration -v
```

Use only a disposable test database. This test creates a temporary table and does not truncate permanent tables. If TEST_DATABASE_URL is empty, it skips explicitly.

## 11. Coverage

```bash
go test -cover ./day-15/testingapi
go test -coverprofile=coverage.out ./day-15/testingapi
go tool cover -func=coverage.out
```

Coverage shows which statements were executed; it does not prove assertions are meaningful. High coverage with weak assertions can still miss bugs. The SQL adapter is intentionally not covered by default tests; run the optional real database test to exercise it.

## 12. Race detection

```bash
go test -race ./day-15/testingapi
```

The race detector finds races exercised by a test run. It cannot prove no race exists on an untested execution path, so include realistic concurrent tests.

## 13. Fuzz testing

```bash
go test -fuzz=FuzzParseProductID -fuzztime=10s ./day-15/testingapi
```

Fuzzing generates many inputs. The sample checks the invariant that successful product-ID parsing always returns a positive integer. Go saves discovered failing inputs in the fuzz corpus so they can be replayed.

## 14. Benchmarks

```bash
go test -bench=BenchmarkProductServiceCreate -benchmem ./day-15/testingapi
```

A benchmark measures performance; `b.ReportAllocs()` reports allocations. Benchmarks are not correctness tests. Compare runs on the same machine and Go version; avoid optimizing from one noisy result.

## 15. Line-by-line walkthrough by file

### product.go
1. Imports provide context, sentinel errors, formatted wrapping and whitespace normalization.
2. Sentinel errors provide stable identities checked with errors.Is.
3. Product is the domain/response model; JSON tags define public field names.
4. CreateProductInput is the request DTO and omits server-owned ID.
5. ProductRepository defines storage behavior without coupling service logic to SQL.
6. ProductService stores the interface; NewProductService injects it.
7. Create trims SKU/name, validates required fields and numeric constraints, and calls storage only after validation.
8. Repository errors are wrapped with %w so errors.Is still works.
9. Get rejects non-positive IDs before storage and adds ID context to errors.
10. List validates limit/offset before querying.

### repository.go
1. MemoryRepository holds a map, ID counter and RWMutex.
2. Constructor initializes the map and starts IDs at one.
3. Create checks cancellation, locks exclusively, allocates and stores a product.
4. Get checks cancellation, takes a read lock and returns ErrNotFound when absent.
5. List takes a read lock, sorts IDs for deterministic output, applies offset/limit and returns an empty non-nil slice when no rows exist.
6. DBTX is satisfied by *sql.DB, *sql.Tx and *sql.Conn.
7. SQLRepository accepts DBTX, which makes it usable with a pool, transaction or dedicated connection.
8. SQL Create uses parameter placeholders and RETURNING to scan generated values.
9. SQL Get maps sql.ErrNoRows to ErrNotFound.
10. SQL List closes rows, scans each record and checks rows.Err.
11. Compile-time interface assignments detect method-signature drift.

### handler.go
1. Imports cover JSON, error inspection, MIME parsing, HTTP, I/O and integer conversion.
2. maxRequestBody is 1 MiB.
3. API depends on ProductService; NewHandler registers method-aware routes.
4. health returns JSON status.
5. createProduct checks Content-Type, strictly decodes JSON, calls service with r.Context, sets Location and returns 201.
6. MaxBytesError maps to 413; malformed/unknown/trailing JSON maps to 400.
7. getProduct parses the route ID, calls the service and returns 200 or a mapped error.
8. listProducts parses pagination, delegates validation and returns an array.
9. parseProductID is a pure function, suitable for table tests and fuzzing.
10. isJSON accepts application/json with optional charset parameters.
11. decodeJSON limits the body, rejects unknown fields and verifies a second decode returns io.EOF.
12. writeServiceError maps domain errors to 422/404 and hides unexpected internal details behind 500.
13. writeJSON sets the content type and writes the status/body.

### main.go
1. Creates a logger and memory repository for a zero-setup demo.
2. Injects repository into service and service into handler.
3. Configures server address and read/write/idle timeouts.
4. signal.NotifyContext watches Ctrl+C/SIGTERM.
5. ListenAndServe runs in a goroutine and reports through a buffered channel.
6. select waits for server failure or shutdown signal.
7. Shutdown uses a fresh ten-second context; Close is last-resort fallback.
8. env reads a setting or returns its default.

### Tests
- service_test.go: fake repository, table-driven validation, trims, wrapped error identity, Get/List arguments and pagination boundaries.
- handler_test.go: HTTP status/header/body contract, strict JSON, body limit, pagination, method handling, error redaction, real local HTTP client via NewServer, t.Helper, t.Cleanup and safe parallel parser subtests.
- postgres_integration_test.go: tagged real database fixture with a temporary table.
- product_fuzz_test.go: seeds plus invariant for arbitrary product IDs.
- benchmark_test.go: service create throughput and allocation measurement.

## 16. Common mistakes

1. Testing private implementation details instead of observable behavior.
2. Using a real database for every unit test.
3. Sharing mutable fixtures between parallel tests.
4. Forgetting to close response bodies or servers.
5. Checking only status and ignoring headers/body.
6. Leaking database errors or secrets in HTTP responses.
7. Treating coverage percentage as proof of correctness.
8. Using time.Sleep instead of deterministic synchronization.
9. Running integration tests against production data.
10. Calling t.Fatal from a worker goroutine.
11. Comparing error strings instead of using errors.Is/errors.As.
12. Writing tests that depend on execution order.

## 17. Exercises and suggested solutions

1. Add a Get test where the fake repository returns an error. **Solution:** configure getFn with a sentinel error and assert errors.Is on the service error.
2. Request limit=abc. **Solution:** expect HTTP 400.
3. Request limit=101. **Solution:** service validation maps to HTTP 422.
4. Send application/json; charset=utf-8. **Solution:** expect 201; mime.ParseMediaType accepts the parameter.
5. Verify an empty list returns [] rather than null. **Solution:** return an empty non-nil slice, decode JSON and assert length zero.
6. Add ErrDuplicateSKU and map it to 409. **Solution:** wrap it in the service and switch using errors.Is.
7. Fuzz the parser for 30 seconds. **Solution:** use -fuzztime=30s and preserve any failing corpus input as a regression test.
8. Benchmark before and after a repository change. **Solution:** run the same benchmark with -benchmem on the same machine.
9. Add duplicate-SKU PostgreSQL coverage. **Solution:** insert the same SKU twice in the temporary table and assert the second insert fails.
10. Test an HTTP client receiving 503. **Solution:** use httptest.NewServer and assert the adapter maps the status as designed.

## 18. Interview questions

1. Unit vs integration test? — Unit isolates a component; integration checks real dependencies/boundaries.
2. Table-driven test? — Named cases executed from a slice.
3. Subtest? — A child test created with t.Run.
4. t.Helper? — Reports failures at the caller of a helper.
5. httptest.NewRecorder? — In-memory ResponseWriter for handler tests.
6. NewRecorder vs NewServer? — Direct handler invocation vs a temporary HTTP endpoint for client tests.
7. Fake repository? — Small test implementation of a repository interface.
8. Why inject interfaces? — Business logic can be tested without concrete storage/network dependencies.
9. errors.Is? — Checks whether an error chain contains a target.
10. Coverage? — Statements executed by tests, not a correctness guarantee.
11. go test -race? — Detects data races exercised by the run.
12. Fuzz test? — Generates inputs to discover crashes/invariant violations.
13. Benchmark? — Measures performance.
14. Build tags for integration tests? — Keep external dependencies out of the default test suite.
15. Why deterministic tests? — Results should not depend on timing or test order.
16. Why not t.Fatal in a goroutine? — Fatal exits only the calling goroutine; report back to the test goroutine.
17. How avoid test pollution? — Isolated fixtures, cleanup, unique data and disposable databases.
18. Why test HTTP headers? — Headers such as Content-Type and Location are part of the API contract.

## 19. Backend use cases

- Verify validation before database writes.
- Test REST status codes and JSON contracts.
- Fake payment, email, SMS and storage providers.
- Check SQL constraints and transactions against real PostgreSQL.
- Test cancellation and timeout behavior.
- Detect concurrent access bugs with race testing.
- Benchmark JSON, repository mapping and hot service paths.
- Fuzz parsers and public input boundaries.

## 20. Day 15 checklist

- [ ] Explain unit, handler and integration tests.
- [ ] Write at least five table-driven cases.
- [ ] Use named subtests.
- [ ] Test an HTTP handler with httptest.
- [ ] Replace a repository with a fake.
- [ ] Verify wrapped errors with errors.Is.
- [ ] Run go test -race and go vet.
- [ ] Generate coverage and explain its limitations.
- [ ] Run a benchmark and a fuzz target.
- [ ] Understand how to run the optional PostgreSQL test safely.

**Next:** Day 16 — Docker, configuration and production runtime foundations.
