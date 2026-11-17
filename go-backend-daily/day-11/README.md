# Day 11 — Middleware, Routing & Request Validation

**Previous lesson:** Day 10 — HTTP Server & REST APIs.  
**Next lesson:** Day 12 — PostgreSQL and SQL Transactions.

## Learning goals
- Understand middleware as reusable code around an HTTP handler.
- Chain middleware in a predictable order.
- Add request IDs, access logs, panic recovery, security headers and Bearer authentication.
- Organize routes by method and resource using Go 1.22+ ServeMux patterns.
- Validate JSON request bodies and return field-level validation errors.
- Test middleware and handlers using `net/http/httptest`.

## Analogy
A request passes through airport checkpoints before reaching its gate: request ID desk, logging desk, recovery layer, security checks, authentication, then the endpoint handler. Each middleware has one focused responsibility and can be reused across routes.

## Middleware concept

```go
func MyMiddleware(next http.Handler) http.Handler {
    return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
        // before the endpoint
        next.ServeHTTP(w, r)
        // after the endpoint
    })
}
```

The signature is `func(http.Handler) http.Handler`. `next` is the next handler. Middleware can inspect or modify the request, add response headers, reject a request early, or perform work after the endpoint returns.

`Chain(final, A, B, C)` makes A the outermost middleware. Request execution is A → B → C → final; response unwinds C → B → A.

## Project files

```text
go-backend-daily/
└── day-11/
    ├── README.md
    └── middlewareapi/
        ├── main.go
        ├── middleware.go
        ├── handlers.go
        └── main_test.go
```

## Run from the go-backend-daily module root

```bash
go run ./day-11/middlewareapi
go test ./day-11/middlewareapi
go test -race ./day-11/middlewareapi
go vet ./day-11/middlewareapi
go build ./day-11/middlewareapi
```

The API listens on `:8080` by default. Set `PORT=18080` to use another port. Set `API_TOKEN` outside local learning. The fallback `dev-token` is deliberately insecure and exists only to make the example easy to run.

## Endpoints

| Method | Path | Auth | Success |
|---|---|---|---|
| GET | `/healthz` | No | 200 |
| GET | `/api/v1/products` | No | 200 |
| POST | `/api/v1/products` | Bearer | 201 |
| GET | `/api/v1/private` | Bearer | 200 |

Create a product:

```bash
curl -i -X POST http://localhost:8080/api/v1/products \
  -H 'Content-Type: application/json' \
  -H 'Authorization: Bearer dev-token' \
  -d '{"name":"Keyboard","price":1499}'
```

List products:

```bash
curl -i http://localhost:8080/api/v1/products
```

Try unauthorized access:

```bash
curl -i http://localhost:8080/api/v1/private
```

## Code walkthrough

### main.go — composition root
1. Creates logger, in-memory store and API handlers.
2. Creates `http.NewServeMux()`.
3. Registers method-aware routes such as `GET /api/v1/products` (Go 1.22+).
4. Wraps protected endpoints with `BearerAuth`.
5. Builds the global chain: RequestID → Logging → Recover → SecurityHeaders → routes.
6. Configures read-header, read, write and idle timeouts.
7. Reads PORT (default 8080) and starts ListenAndServe.

### middleware.go — cross-cutting behavior
- `contextKey` and `requestIDKey`: private typed context key.
- `Chain`: reverse loop makes the first middleware outermost.
- `newRequestID`: generates a random diagnostic identifier.
- `RequestID`: adds `X-Request-ID` and puts the ID in request context.
- `statusWriter`: tracks response status and bytes written.
- `Logging`: records method, path, status, bytes, duration and request ID.
- `Recover`: catches a panic in the same goroutine and writes a generic 500 response.
- `SecurityHeaders`: adds baseline browser security headers.
- `BearerAuth`: checks `Authorization: Bearer <token>` and stops unauthorized requests. The sample uses constant-time comparison for token bytes.

### handlers.go — endpoints and validation
- `Product` is the response/domain shape; `CreateProductRequest` is the input DTO.
- `FieldError` and `ValidationError` represent field-specific errors.
- `Validate` trims the name and checks its length and price range.
- `ProductStore` protects the shared slice and next ID with `sync.RWMutex`.
- `decodeRequestJSON` checks media type, caps the body at 1 MiB, rejects unknown fields and rejects trailing JSON values.
- `createProduct` maps malformed input to 400, unsupported content type to 415, oversized bodies to 413, business validation to 422, and success to 201.
- The `Location` response header points to the created resource.

### main_test.go — tests
Uses `httptest.NewRequest` and `httptest.NewRecorder` to test handlers without opening a TCP port. Tests cover request IDs, security headers, authentication, create/list, validation, strict JSON decoding, media type, oversized bodies, panic recovery, middleware order and unique request IDs.

## Important design notes
- Middleware order matters. Logging outside authentication records rejected attempts; this project logs all requests.
- Request IDs are for correlation, not authentication.
- `recover` only catches panics in the same goroutine. A panic in a separately launched goroutine needs recovery in that goroutine.
- A simple `ResponseWriter` wrapper can affect optional interfaces such as `http.Flusher`, `http.Hijacker` and `http.Pusher`. This sample is for ordinary JSON APIs, not streaming/WebSocket endpoints. Production middleware should preserve optional interfaces or use a tested wrapper.
- The demo token fallback must never be used in production. Use secret management, rotation, TLS, expiry/signature verification and authorization policy.
- Authentication answers “who are you?”; authorization answers “are you allowed to do this?”.
- The in-memory store loses data on restart and is not shared between replicas. Day 12 introduces PostgreSQL and transactions.
- The sample's constant-time token comparison is a small improvement, not a complete identity system.

## Exercises

1. Add an `X-Response-Time` header middleware. Capture the start time, call next, then set the elapsed time. Think about when headers are committed; setting it after next returns is too late if the handler already wrote the response. Improve it by wrapping WriteHeader and setting the header before forwarding the first status.
2. Add middleware that rejects requests without `X-Client-Version` for `/api/v1/*` only.
3. Add validation that product name cannot contain only punctuation.
4. Add `GET /api/v1/products/{id}` and return 404 for unknown IDs.
5. Change BearerAuth to read a token from configuration and fail startup if production configuration is empty.
6. Add a test proving each request gets a different request ID.
7. Add a panic test verifying the response is generic and does not expose the panic detail.

### Suggested solutions
1. Use a response-writer wrapper whose WriteHeader sets X-Response-Time before forwarding the first status.
2. Wrap only the versioned sub-mux or the individual versioned routes.
3. Iterate over runes and require at least one `unicode.IsLetter` or `unicode.IsDigit` rune.
4. Read `r.PathValue("id")`, parse with `strconv.Atoi`, query a synchronized store and return 404 when absent.
5. Load API_TOKEN from a secret manager/environment and fail startup if missing in production. Never commit real tokens.
6. Perform two health requests and compare their non-empty X-Request-ID headers.
7. Assert status 500 and generic error text; ensure the panic value is absent from the body.

## Interview questions and answers

1. **What is middleware?** A reusable handler wrapper that runs before/after another handler.
2. **What is its signature?** `func(http.Handler) http.Handler`.
3. **Why does order matter?** Outer middleware runs first on the request and last on the response.
4. **What does next.ServeHTTP do?** Passes the request to the next handler.
5. **Why request IDs?** Correlate one request across logs and downstream services.
6. **What can recover catch?** A panic unwinding in the same goroutine, not another goroutine.
7. **401 vs 403?** 401 means missing/invalid authentication; 403 means authenticated but not allowed.
8. **400 vs 422?** 400 commonly means malformed input; 422 means syntactically understood content failed semantic validation.
9. **Why MaxBytesReader?** Caps request-body size to protect memory and CPU.
10. **Why sync.RWMutex?** Coordinates concurrent readers and writers of shared state.
11. **What are method-aware ServeMux patterns?** Go 1.22+ patterns such as `GET /products/{id}`.
12. **Is a static Bearer token enough for production?** No. Production needs secure secret storage, rotation, expiry/verification, authorization policy, TLS and auditability.
13. **Does recover replace normal error handling?** No. Expected failures should be returned as errors and mapped to suitable HTTP responses.
14. **Why not log all request bodies?** Bodies may contain passwords, tokens or personal data; redact and log only what is needed.

## Backend use cases
- Request ID: trace a support issue through API and worker logs.
- Logging: monitor endpoint latency and status codes.
- Recovery: isolate an unexpected handler panic from the server process.
- Authentication: protect create/admin endpoints.
- Validation: return field-level errors to Angular reactive forms.
- Security headers: set baseline browser protections.
- Route grouping: version endpoints under `/api/v1`.

## Production checklist
- [ ] Set a request-body size limit.
- [ ] Use HTTPS/TLS in deployment.
- [ ] Keep secrets out of source code and logs.
- [ ] Never deploy the demo token.
- [ ] Separate authentication from authorization.
- [ ] Decide middleware order intentionally.
- [ ] Return generic 500 responses; log details privately.
- [ ] Do not log sensitive request bodies.
- [ ] Preserve optional ResponseWriter interfaces for streaming/WebSockets.
- [ ] Use a database rather than an in-memory slice for durable multi-instance storage.

## Next lesson
**Day 12 — PostgreSQL and SQL Transactions:** connect Go to PostgreSQL, configure a connection pool, execute parameterized SQL, use BEGIN/COMMIT/ROLLBACK, and test repository code.
