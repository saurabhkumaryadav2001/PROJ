# Day 10 — HTTP Server & REST APIs (Beginner → Advanced)

**Sequence:** Day 09 JSON/File I/O → **Day 10 HTTP & REST** → Day 11 Middleware, routing and request validation.

Today turns yesterday's JSON knowledge into a real HTTP API. This example is in-memory so HTTP is learned before Day 12 PostgreSQL/transactions.

## Learning outcomes
- Explain client → HTTP request → handler → response.
- Build routes with Go 1.22+ method-aware ServeMux patterns.
- Implement CRUD and choose correct status codes.
- Decode strict JSON, cap request bodies and validate business input.
- Use a concurrency-safe Store with sync.RWMutex.
- Return consistent JSON and Location on creation.
- Configure server timeouts and graceful shutdown.
- Test handlers using httptest without opening a real port.

## 1. Mental model: a restaurant order counter
- Client (Angular/curl): customer placing an order.
- HTTP request: order slip (method, URL, headers, body).
- Router (ServeMux): sends the slip to the correct counter.
- Handler: employee who understands that operation.
- Store/service: kitchen or inventory.
- HTTP response: receipt (status, headers, optional JSON body).

A handler has this shape:
```go
func(w http.ResponseWriter, r *http.Request)
```
`r` is the incoming request; `w` constructs the response. Do not print API output using fmt.Println—write it to w.

## 2. REST routes
| Method | URL | Meaning | Success |
|---|---|---|---|
| GET | /healthz | Health check | 200 |
| GET | /products | List | 200 |
| POST | /products | Create | 201 + Location |
| GET | /products/{id} | Read one | 200 |
| PUT | /products/{id} | Replace editable fields | 200 |
| DELETE | /products/{id} | Delete | 204, no body |

Error contract:
- 400 Bad Request: malformed JSON, unknown field, invalid ID/input.
- 404 Not Found: valid ID, resource absent.
- 405 Method Not Allowed: path exists but method unsupported (ServeMux).
- 413 Content Too Large: body exceeds 1 MiB.
- 415 Unsupported Media Type: not application/json.
- 500 Internal Server Error: unexpected server failure.

Status codes are part of the API contract; do not return 200 for every outcome.

## 3. DTO, validation and Store
`Product` is the response/domain shape (ID, Name, Price, CreatedAt). `CreateProductRequest` contains only Name and Price. The client cannot choose ID or CreatedAt; the server owns those fields.

`Validate` trims Name, rejects blank names, price <= 0 and price > 1,000,000. JSON decoding checks shape; validation checks business rules. Keep these separate.

The in-memory Store uses a map and `sync.RWMutex`:
- RLock/RUnlock for List/Get.
- Lock/Unlock for Create/Update/Delete.
- defer Unlock releases the lock on every return.
- List copies map values into a sorted slice; it does not expose the map.

This demo is concurrency-safe, but data disappears on restart, is process-local, and has no database transactions. Day 12 replaces it with PostgreSQL.

## 4. Project files
```
go-backend-daily/
├── go.mod
└── day-10/
    ├── README.md
    └── restapi/
        ├── main.go
        └── main_test.go
```
The existing module is github.com/saurabhkumaryadav2001/go-backend-daily (Go 1.23). Do not add a nested go.mod.

## 5. Source walkthrough
Read `restapi/main.go` top-to-bottom; this section maps each logical block:

### Imports and limit
- context: shutdown deadline.
- encoding/json: request decode and response encode.
- errors: errors plus errors.As/Is.
- fmt: Location and wrapped errors.
- io: require second decode to reach EOF.
- log: lifecycle/encoding diagnostics.
- mime: parse Content-Type correctly (including charset).
- net/http: server, router, handler, status and body limit.
- os/os-signal/syscall: environment and shutdown signals.
- sort: deterministic list order (map order is random).
- strconv: parse URL IDs.
- strings: trim and inspect error text.
- sync: protect shared map.
- time: timestamp and timeouts.
- maxRequestBody = 1<<20 = 1 MiB.

### Product and request DTO
Product uses JSON tags. CreateProductRequest excludes server-owned ID/time. Validate trims name and enforces required name and price range.

### Store
- NewStore creates map and nextID=1.
- List: read lock, copy values, sort by ID, return slice.
- Get: read lock, return product + found flag.
- Create: write lock, assign ID and UTC CreatedAt, store, increment ID.
- Update: write lock, check existence, change Name/Price only, preserve ID/CreatedAt.
- Delete: write lock, delete and return whether it existed.

### Router and handlers
`NewHandler` creates ServeMux and registers method-aware patterns. Go 1.22+ supports patterns like `GET /products/{id}`; `r.PathValue("id")` reads the wildcard.
- health: 200 + {"status":"ok"}.
- listProducts: 200 + product array (empty is [] rather than null).
- createProduct: decode → validate → create → Location header → 201.
- getProduct: parse ID → Get → 400 invalid, 404 missing, 200 found.
- updateProduct: parse ID → decode/validate → update → 404 missing, 200 updated.
- deleteProduct: parse ID → delete → 404 missing, 204 with no body.
- parseID: strconv.Atoi and require positive integer.

### Strict decodeJSON
1. mime.ParseMediaType accepts application/json and charset parameters, but not application/jsonp.
2. http.MaxBytesReader caps request at 1 MiB.
3. json.NewDecoder reads the stream.
4. DisallowUnknownFields catches misspellings such as prcie.
5. First Decode populates DTO.
6. Second Decode must return io.EOF; otherwise reject a second JSON value/trailing content.
7. Validate is still required because missing JSON fields become Go zero values.

writeDecodeError maps *http.MaxBytesError to 413, media type to 415, other decode failures to 400.

### Response helpers
- writeJSON sets Content-Type before WriteHeader, then encodes JSON.
- writeError uses {"error":"..."} consistently.
- 204 is written without a body.

### Server lifecycle
PORT environment variable defaults to 8080. http.Server sets ReadHeaderTimeout, ReadTimeout, WriteTimeout and IdleTimeout to limit slow clients. signal.NotifyContext listens for Ctrl+C/SIGTERM. ListenAndServe runs in a goroutine. On signal, Shutdown stops new requests and waits for in-flight work up to 10 seconds; Close is fallback. http.ErrServerClosed is expected during graceful shutdown.

## 6. Run and test
From module root:
```bash
go run ./day-10/restapi
go test ./day-10/restapi
go test -race ./day-10/restapi
go vet ./day-10/restapi
```

## 7. curl examples
```bash
curl -i http://localhost:8080/healthz

curl -i -X POST http://localhost:8080/products \
  -H "Content-Type: application/json" \
  -d '{"name":"Keyboard","price":1499}'

curl -i http://localhost:8080/products
curl -i http://localhost:8080/products/1

curl -i -X PUT http://localhost:8080/products/1 \
  -H "Content-Type: application/json" \
  -d '{"name":"Mechanical Keyboard","price":1999}'

curl -i -X DELETE http://localhost:8080/products/1
```
Create returns 201, Location: /products/1 and a JSON product. Empty name/negative price → 400; text/plain → 415; missing ID → 404.

## 8. Exercises + solution directions
1. Add GET /products/{id}/price: parse ID, Get, return small DTO or 404.
2. Add PATCH for name only: use Name-only DTO and Store method. Do not set omitted price to zero; model field presence deliberately.
3. Add ?limit=: parse query, accept 1..100, 400 otherwise, slice after validation.
4. Test unsupported PATCH: ServeMux returns 405 when path matches but method does not.
5. Explain mutex: handlers run concurrently; map access must be synchronized. Run go test -race.
6. Make Store injectable: define a small ProductRepository interface near API consumer; later implement with SQL.

## 9. Interview questions
1. **HTTP handler?** Function or type implementing ServeHTTP(ResponseWriter, *Request).
2. **ResponseWriter vs Request?** Writer builds response; Request contains inbound method, URL, headers, body, context and path values.
3. **REST?** Resource-oriented style using HTTP semantics; JSON alone is not REST.
4. **PUT vs PATCH?** PUT generally replaces representation; PATCH applies partial changes.
5. **200/201/204?** Success; resource created; success with no body.
6. **400 vs 404?** Invalid request vs absent resource.
7. **MaxBytesReader?** Limits payload size/resource consumption.
8. **Second Decode?** Reject multiple JSON documents/trailing content.
9. **RWMutex?** Protect concurrent map access while allowing parallel readers.
10. **Graceful shutdown?** Stop new requests and let in-flight work finish within deadline.
11. **Timeouts?** Prevent slow clients holding connections indefinitely.
12. **Test handlers?** httptest.NewRecorder/NewRequest; httptest.NewServer for client-level tests.
13. **Why not production persistence?** Volatile, process-local, no replica sharing/durability/transactions.
14. **Middleware?** Handler wrapper for cross-cutting concerns; Day 11.

## 10. Backend use cases
- Angular/React CRUD clients.
- Health endpoint for load balancers/orchestrators.
- Location header to discover created resource.
- Consistent JSON error shape for frontend.
- Body limits/timeouts protect availability.
- Mutex makes local demo safe under concurrent requests.
- Graceful shutdown during deployment.
- Production additions: DB, auth, logs/metrics/tracing, pagination, request IDs.

## Production checklist
- [ ] Correct method/resource routes and status codes
- [ ] Consistent JSON errors
- [ ] Content-Type and body size limits
- [ ] Strict decoding policy + separate business validation
- [ ] Shared state synchronized or database-backed
- [ ] Server timeouts and graceful shutdown
- [ ] Unit/integration tests and race test
- [ ] Add authentication, authorization, observability, DB and rate limits before public exposure

**Takeaway:** HTTP is the envelope; JSON is the payload. Route → decode → validate → business/store → response. Keep transport concerns at the edge.

**Next: Day 11 — Middleware, routing and request validation** (request IDs, logging, recovery, auth middleware and reusable validation patterns).
