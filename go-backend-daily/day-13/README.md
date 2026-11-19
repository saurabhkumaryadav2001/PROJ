# Day 13 — Context, Cancellation & Timeouts in Go

**Previous:** Day 12 — PostgreSQL, database/sql, connection pooling and transactions  
**Next:** Day 14 — Goroutines, channels, synchronization and worker pools

## Learning goals

- Explain Deadline, Done, Err and Value on context.Context.
- Use Background, TODO, WithCancel, WithTimeout and WithDeadline correctly.
- Propagate HTTP request context through handlers, services, SQL and outbound HTTP.
- Distinguish context.Canceled from context.DeadlineExceeded.
- Understand parent/child cancellation and earliest-deadline inheritance.
- Use context values only for small request-scoped metadata.
- Stop goroutines cooperatively and prevent goroutine leaks.
- Run bounded concurrent work and wait for workers to exit.
- Shut down an HTTP server gracefully with a separate deadline.

## 1. Why context matters

Imagine an Angular client calls a Go API. The API starts a database query and then calls another service. The user closes the browser tab while those operations are still running. Without cancellation propagation, the API may continue consuming goroutines, database connections, CPU and downstream capacity for work nobody needs.

A context is a request-scoped signal that carries cancellation, a deadline and small request metadata through a call chain.

Analogy: a manager gives a team one job ticket with a deadline. If the customer cancels the job or the deadline expires, every team member checks the ticket and stops. The manager does not forcibly kill a worker; each worker must cooperate.

## 2. The Context interface

~~~go
type Context interface {
    Deadline() (deadline time.Time, ok bool)
    Done() <-chan struct{}
    Err() error
    Value(key any) any
}
~~~

- Deadline reports the deadline, if one exists.
- Done is closed when the context is canceled or expires.
- Err returns context.Canceled or context.DeadlineExceeded after cancellation.
- Value retrieves request-scoped metadata.

Pass context explicitly, normally as the first argument:

~~~go
func LoadCustomer(ctx context.Context, id int64) (Customer, error)
~~~

Do not store request contexts in long-lived structs or pass nil. Use Background only at a true process/test root. Use TODO temporarily when a context decision has not yet been made.

## 3. Parent and child contexts

~~~go
root := context.Background()
ctx, cancel := context.WithCancel(root)
defer cancel()

child, cancelChild := context.WithTimeout(ctx, 2*time.Second)
defer cancelChild()
~~~

- WithCancel is manually canceled.
- WithTimeout adds a relative deadline.
- WithDeadline adds an absolute deadline.
- Parent cancellation propagates to descendants.
- Canceling a child does not cancel its parent or siblings.
- A child cannot extend an earlier parent deadline; the earliest deadline wins.
- Always call the returned cancel function, including when the operation succeeds early.

Cancellation is cooperative. It does not kill a goroutine. A goroutine must select on Done, use a context-aware API, or periodically check Err.

## 4. Project structure

~~~text
go-backend-daily/
└── day-13/
    ├── README.md
    └── contextapi/
        ├── main.go
        ├── context.go
        ├── handlers.go
        ├── work.go
        └── main_test.go
~~~

This lesson uses only the Go standard library. From the go-backend-daily module root:

~~~bash
go run ./day-13/contextapi
go test ./day-13/contextapi
go test -race ./day-13/contextapi
go vet ./day-13/contextapi
go build -o context-api ./day-13/contextapi
~~~

Set a different port in PowerShell if 8080 is occupied:

~~~powershell
$env:PORT = "8090"
go run ./day-13/contextapi
~~~

## 5. File-by-file and line-by-line walkthrough

### contextapi/main.go — process root and graceful shutdown

1. Background creates the root context for the process.
2. signal.NotifyContext derives a context canceled on Ctrl+C or SIGTERM.
3. ListenAndServe runs in a goroutine so main can wait for a server error or shutdown signal.
4. On shutdown, a new Background-derived context gets a ten-second timeout. Do not derive shutdown from the already-canceled signal context.
5. Server.Shutdown stops accepting new connections and waits for active handlers until completion or deadline.
6. Server.Close is the last-resort force close if graceful shutdown exceeds its deadline.
7. ReadHeaderTimeout, ReadTimeout, WriteTimeout and IdleTimeout protect the server from slow clients. They are different from an operation-level context deadline.

### contextapi/context.go — request ID and logging

- contextKey is a private key type so other packages cannot accidentally collide.
- RequestIDFromContext retrieves the request ID and checks its type.
- RequestID uses an atomic sequence for this teaching example, stores the ID with context.WithValue, sets X-Request-ID and calls the next handler with r.WithContext(ctx).
- Logging measures request duration and logs the ID plus the final context error.
- Middleware order is RequestID(Logging(mux)): RequestID is outermost and adds the value before Logging runs.
- A request ID is correlation metadata, not authentication. Production IDs should meet trace/correlation requirements and must never be treated as secrets.

### contextapi/work.go — cancellation-aware work and worker pool

waitFor creates a timer and selects between the timer and ctx.Done. If the timer fires, work completes. If the context is canceled, it returns ctx.Err immediately. The timer is stopped with defer.

RunTasks demonstrates bounded concurrency:

1. Validate worker count and task inputs before starting goroutines.
2. Derive workCtx from the caller's context.
3. Create a jobs channel and a results channel buffered to the number of tasks.
4. Start no more than workerCount workers.
5. Each worker selects on workCtx.Done and jobs.
6. Each simulated task calls waitFor(workCtx, duration).
7. The producer also selects on cancellation while sending jobs.
8. Close jobs, wait for every worker, close results and collect completed results.
9. Return the caller's context error if cancellation occurred.
10. Defer cancel to release child-context resources.

Result order is completion order, not input order. If input order matters, include an index and reorder the results.

### contextapi/handlers.go — request propagation

- net/http provides r.Context for each incoming request.
- The server cancels it when the client connection closes, an HTTP/2 request is canceled, or the handler returns.
- GET /api/v1/work passes r.Context into waitFor.
- GET /api/v1/timeout derives a 250ms child timeout and simulates a two-second operation; expected response is 504.
- POST /api/v1/batch passes the request context into RunTasks, so workers stop when the request is canceled.
- DeadlineExceeded maps to 504; Canceled maps to 408 for the demonstration. A disconnected client may no longer receive a response.
- The batch body is limited to 64 KiB; unknown JSON fields and invalid task counts/durations are rejected.

Never replace r.Context with Background inside a request handler. That detaches work from client cancellation.

### contextapi/main_test.go — tests

Tests cover prompt cancellation, deadline expiry, child/parent behavior, inherited deadlines, request ID propagation, the 504 timeout endpoint, bounded batch completion, worker cancellation and unknown JSON-field rejection.

## 6. Try the API

Start:

~~~bash
go run ./day-13/contextapi
~~~

Health:

~~~bash
curl -i http://localhost:8080/healthz
~~~

Request ID and context metadata:

~~~bash
curl -i http://localhost:8080/api/v1/request-info
~~~

Normal cancellable work:

~~~bash
curl -i "http://localhost:8080/api/v1/work?duration=300ms"
~~~

Child timeout; expected HTTP 504:

~~~bash
curl -i http://localhost:8080/api/v1/timeout
~~~

Bounded worker batch:

~~~bash
curl -i -X POST http://localhost:8080/api/v1/batch \
  -H "Content-Type: application/json" \
  -d '{"tasks":[{"id":"inventory","duration_ms":300},{"id":"pricing","duration_ms":500},{"id":"customer","duration_ms":200}]}'
~~~

Try starting a five-second work request and stop curl with Ctrl+C. The server should log context cancellation when the disconnect is detected. Exact timing depends on the client and any proxy.

## 7. Backend patterns

### PostgreSQL query (connects to Day 12)

~~~go
func (r *SQLRepository) GetProduct(ctx context.Context, id int64) (Product, error) {
    queryCtx, cancel := context.WithTimeout(ctx, 2*time.Second)
    defer cancel()

    var product Product
    err := r.db.QueryRowContext(queryCtx,
        `SELECT id, sku, name, price_paise, stock FROM products WHERE id = $1`,
        id,
    ).Scan(&product.ID, &product.SKU, &product.Name, &product.PricePaise, &product.Stock)
    return product, err
}
~~~

Use ExecContext, QueryContext, QueryRowContext and BeginTx with the request context. The driver/database must support cancellation for the query to stop promptly.

### Outbound HTTP call

~~~go
req, err := http.NewRequestWithContext(ctx, http.MethodGet, serviceURL, nil)
if err != nil { return err }
resp, err := httpClient.Do(req)
if err != nil { return err }
defer resp.Body.Close()
~~~

The outgoing request is canceled when its context is canceled. Configure a reasonable client timeout as defense in depth.

### Avoid untracked goroutines

Bad: start a goroutine with no cancellation path and no way to wait for it.

Better: pass a context, select on ctx.Done, and use a WaitGroup or managed worker lifecycle so shutdown waits for the goroutine to exit.

## 8. Advanced APIs and mistakes

### Canceled vs deadline exceeded

- context.Canceled means the caller or a parent canceled the context.
- context.DeadlineExceeded means its deadline elapsed.
- Use errors.Is, not error-string comparisons.

### WithCancelCause (Go 1.20+)

~~~go
ctx, cancel := context.WithCancelCause(parent)
cancel(errors.New("deployment is shutting down"))
fmt.Println(ctx.Err())          // context canceled
fmt.Println(context.Cause(ctx)) // deployment is shutting down
~~~

### WithoutCancel (Go 1.21+)

It keeps parent values but does not inherit cancellation or deadline. Use only for bounded work that intentionally must outlive the request, such as a short audit write. Add a new timeout; never detach work indefinitely.

### AfterFunc (Go 1.21+)

Schedules a function after cancellation. The callback runs asynchronously; synchronize shared state carefully.

### Context values

Use values for small cross-cutting metadata: request IDs, trace IDs or authenticated-principal metadata. Do not use them for optional parameters, database handles, logger configuration or business inputs. Use a private key type.

### Common mistakes

1. Forgetting to call cancel.
2. Creating Background inside a request handler.
3. Assuming cancel forcibly kills a goroutine.
4. Ignoring Done in long-running loops.
5. Not waiting for goroutines before closing resources.
6. Giving each downstream call a fresh full timeout rather than respecting the caller's remaining budget.
7. Holding a transaction open during a slow external API call.
8. Putting large objects in context values.
9. Treating a request ID as authentication.
10. Assuming a disconnected client receives a response.

## 9. Exercises

1. Add a 750ms service timeout and map DeadlineExceeded to 504.
2. Update a Day 12 repository method to accept ctx and use QueryRowContext with a two-second child timeout.
3. Cancel an outbound HTTP request with NewRequestWithContext and an httptest server.
4. Add a 1.5-second batch deadline derived from r.Context; the earlier parent deadline must win.
5. Add a long-lived worker canceled on SIGTERM and wait for it with a WaitGroup.
6. Use WithCancelCause and log both ctx.Err and context.Cause.
7. Test that every worker exits after parent cancellation.

### Suggested solutions

1. Derive ctx with WithTimeout(r.Context(), 750*time.Millisecond), defer cancel, run the operation and map DeadlineExceeded to 504.
2. Derive queryCtx from the received ctx; call QueryRowContext(queryCtx, query, id).
3. Create the outgoing request with NewRequestWithContext; have the test server wait on r.Context().Done.
4. Derive the child from r.Context, never Background.
5. Use signal.NotifyContext, pass it to the worker, cancel on signal and wait for the worker.
6. Call cancel(errors.New("queue closed")); log ctx.Err and context.Cause(ctx).
7. Have each worker defer wg.Done, select on ctx.Done, cancel and wait with a bounded test deadline.

## 10. Interview questions

1. What is context.Context? — A standard mechanism for deadlines, cancellation and request metadata.
2. What does Done return? — A channel closed when canceled or expired.
3. Does cancel kill a goroutine? — No; the goroutine must cooperate.
4. WithCancel vs WithTimeout? — Manual cancellation vs automatic deadline.
5. What happens when a parent is canceled? — Descendants are canceled.
6. Can a child extend its parent's deadline? — No; the earliest deadline wins.
7. Why call cancel after success? — To release timer and parent-child resources promptly.
8. What cancels an incoming HTTP request context? — Client connection close, HTTP/2 request cancellation or handler return.
9. Why QueryContext instead of Query? — It allows request cancellation/deadline to reach the database operation.
10. Canceled vs DeadlineExceeded? — Cancellation signal vs elapsed deadline.
11. Should context be stored in a struct? — Generally no; pass it explicitly.
12. What belongs in context values? — Small cross-cutting metadata.
13. How prevent goroutine leaks? — Cancellation, Done checks, channel lifecycle and waiting for workers.
14. Why separate graceful-shutdown context? — The signal context is already canceled.
15. What is WithCancelCause? — Cancellation with an additional cause available through context.Cause.

## 11. Quality checks

~~~bash
gofmt -w day-13/contextapi/*.go
go test ./day-13/contextapi
go test -race ./day-13/contextapi
go vet ./day-13/contextapi
go build -o ./bin/context-api ./day-13/contextapi
~~~

The project uses only the standard library; no database or external service is needed.

## 12. Production checklist

- [ ] Pass r.Context from handler to service to repository.
- [ ] Use context-aware database and HTTP methods.
- [ ] Set realistic per-operation timeouts inside the caller's deadline budget.
- [ ] Call every cancel function.
- [ ] Make every long-running goroutine observe cancellation.
- [ ] Wait for workers before closing shared resources.
- [ ] Keep context values limited to request metadata.
- [ ] Bound graceful shutdown with a separate timeout.
- [ ] Log cancellation/deadline outcomes without secrets.
- [ ] Test cancellation and goroutine cleanup.

## Next lesson

**Day 14 — Goroutines, Channels, Synchronization & Worker Pools:** goroutine lifecycle, buffered/unbuffered channels, select, WaitGroup, Mutex/RWMutex, bounded concurrency, fan-out/fan-in, backpressure and race-free shared state.
