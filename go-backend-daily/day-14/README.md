# Day 14 — Goroutines, Channels, Synchronization & Worker Pools

**Previous:** Day 13 — Context, cancellation, timeouts and graceful shutdown  
**Next:** Day 15 — Testing Go Backends

This lesson uses Day 13 context only as a cooperative cancellation signal. The new focus is Go concurrency: goroutines, channels, select, WaitGroup, mutexes, fan-out/fan-in, backpressure and bounded worker pools.

## Learning goals

- Explain goroutines and their difference from OS threads.
- Understand the G-M-P scheduler at a practical level.
- Create, send to, receive from and close channels safely.
- Compare buffered and unbuffered channels.
- Use select to wait on work, results and cancellation.
- Coordinate goroutine lifetimes with sync.WaitGroup.
- Protect shared state with sync.Mutex and sync.RWMutex.
- Recognize data races, deadlocks, goroutine leaks and unbounded concurrency.
- Build a bounded worker pool with backpressure.
- Preserve input order when workers finish out of order.

## 1. Why concurrency matters

Imagine an API receives 80 independent price calculations. If each task needs 100ms and runs sequentially, the batch can take roughly eight seconds. Concurrent workers let independent tasks make progress at the same time.

But launching 80 goroutines for every request can overload a database, partner API or the server. Production systems need bounded concurrency: run a fixed number of workers and queue only a limited number of pending jobs.

Analogy: a restaurant has four cooks and a rack with eight waiting orders. When the rack is full, the waiter waits before accepting more work. That is backpressure.

Today's project is a quote API. It accepts product items, calculates subtotals concurrently and returns results in request order.

## 2. Goroutines and the G-M-P scheduler

Start a goroutine with the go keyword:

~~~go
go doWork()
~~~

The call returns immediately; doWork runs concurrently. A goroutine is not an OS thread. The Go runtime multiplexes many goroutines over OS threads.

A useful model:
- G = goroutine, the work and its execution state.
- M = machine, an operating-system thread.
- P = processor scheduling resource needed by an M to execute Go code.

GOMAXPROCS controls how many Ps can execute Go code simultaneously. It is not a maximum goroutine count.

Concurrency means tasks make progress during overlapping periods. Parallelism means tasks execute at the same instant on different CPU cores. I/O-bound work often benefits from concurrency without requiring CPU parallelism.

## 3. Channels

A channel is a typed conduit between goroutines.

~~~go
jobs := make(chan int) // unbuffered
jobs <- 42
value := <-jobs
~~~

An unbuffered send waits for a receiver; the receive waits for a sender. This is a synchronization hand-off.

A buffered channel has a finite queue:

~~~go
jobs := make(chan int, 8)
jobs <- 1
~~~

It can absorb a short burst while space remains. When full, the sender blocks. A buffer is not unlimited capacity.

Receive with the two-value form:

~~~go
value, ok := <-jobs
if !ok {
    // channel closed and drained
}
~~~

A range loop receives until close:

~~~go
for job := range jobs {
    process(job)
}
~~~

### Channel ownership rules

1. The sender or component that owns sending normally closes the channel.
2. Never send to a closed channel; it panics.
3. Closing twice panics.
4. Closing does not discard buffered values.
5. A nil channel blocks forever on send and receive.
6. In this project, the producer closes jobs. A coordinator closes results only after every worker has stopped.

## 4. Select

Select waits for whichever channel operation is ready:

~~~go
select {
case job := <-jobs:
    process(job)
case <-ctx.Done():
    return
case results <- result:
    // delivered
}
~~~

If multiple cases are ready, Go chooses one pseudo-randomly. Do not assume case order means priority. A default case makes select non-blocking; a default in a worker loop can accidentally create a CPU-burning busy loop.

## 5. WaitGroup

WaitGroup waits for a group of goroutines:

~~~go
var wg sync.WaitGroup
wg.Add(3)
for i := 0; i < 3; i++ {
    go func() {
        defer wg.Done()
        doWork()
    }()
}
wg.Wait()
~~~

Call Add before starting goroutines. Every Add(1) needs exactly one Done. Defer Done at goroutine entry so early returns are covered. Do not copy a WaitGroup after first use.

WaitGroup does not transfer data or cancel work. Channels communicate; context signals cancellation; WaitGroup waits for completion.

## 6. Mutex and RWMutex

A data race occurs when concurrent goroutines access the same memory, at least one access is a write, and there is no synchronization.

Use Mutex for exclusive access:

~~~go
type Counter struct {
    mu sync.Mutex
    value int
}
func (c *Counter) Increment() {
    c.mu.Lock()
    defer c.mu.Unlock()
    c.value++
}
~~~

RWMutex allows multiple readers or one writer:

~~~go
type Config struct {
    mu sync.RWMutex
    values map[string]string
}
func (c *Config) Get(key string) string {
    c.mu.RLock()
    defer c.mu.RUnlock()
    return c.values[key]
}
~~~

Use RWMutex only when the read/write workload justifies it. A plain Mutex is often simpler. Do not return a mutable map or slice from protected state without copying it or defining ownership.

The Metrics type in this project uses RWMutex: request handlers update counters under Lock; the metrics endpoint reads a consistent snapshot under RLock.

## 7. Fan-out, fan-in and backpressure

- Fan-out: one producer distributes jobs to multiple workers.
- Fan-in: workers send outcomes to one results channel/collector.
- Backpressure: when the bounded jobs queue is full, the producer waits rather than allocating unlimited work.

~~~text
HTTP request
    |
 producer ---- jobs channel ----+---- worker 1 ----+
                                +---- worker 2 ----+---- results ---> ordered collector
                                +---- worker 3 ----+
                                +---- worker 4 ----+
~~~

Important production detail: a four-worker pool created separately for each request does not cap the whole process at four workers. If 100 requests arrive concurrently, there may be up to 400 workers. Use a process-wide semaphore or shared queue when a global downstream limit is required.

## 8. Project structure

~~~text
go-backend-daily/
  go.mod
  day-14/
    README.md
    concurrencyapi/
      main.go
      pool.go
      metrics.go
      handlers.go
      main_test.go
~~~

The project uses only the Go standard library. From the go-backend-daily module root:

~~~bash
gofmt -w day-14/concurrencyapi/*.go
go test ./day-14/concurrencyapi
go test -race ./day-14/concurrencyapi
go vet ./day-14/concurrencyapi
go build -o ./bin/concurrency-api ./day-14/concurrencyapi
go run ./day-14/concurrencyapi
~~~

PowerShell:

~~~powershell
$env:WORKERS = "4"
$env:QUEUE_SIZE = "8"
$env:PORT = "8090"
go run ./day-14/concurrencyapi
~~~

## 9. File-by-file explanation

### concurrencyapi/pool.go

- QuoteItem is one task; ProcessingMS simulates an I/O wait and is capped by the handler.
- Quote is the result DTO. Currency is stored as integer paise.
- indexedItem carries the original request index because completion order can differ from input order.
- quoteOutcome carries either a result or an error.
- NewQuoteService validates worker and queue limits.
- Calculate checks cancellation and handles an empty batch.
- workerCount is the smaller of configured workers and number of items.
- jobs is a bounded channel. results can hold one outcome per item.
- WaitGroup.Add is called before goroutines start; every worker defers Done.
- Workers select between cancellation and receiving jobs.
- calculateOne uses a timer and selects on ctx.Done so cancellation interrupts simulated work.
- The producer is the only sender to jobs and owns close(jobs).
- A coordinator waits for every worker and then closes results. This prevents send-after-close panics.
- The collector stores each result at its original index, preserving request order.
- Multiplication is checked before calculating the subtotal to prevent int64 overflow.

### concurrencyapi/metrics.go

Metrics owns counters protected by RWMutex. BeginRequest and EndRequest take the exclusive lock. Snapshot uses RLock to copy a consistent view. The mutex and counters are private.

### concurrencyapi/handlers.go

The handler checks Content-Type, caps the request body at 64 KiB, rejects unknown JSON fields and trailing JSON, and validates 1–100 items. It derives a five-second context from r.Context so an earlier client deadline still wins. The pool is independent of HTTP and returns domain errors. The handler maps cancellation/deadline/validation errors to HTTP statuses.

### concurrencyapi/main.go

WORKERS controls the per-request worker count (default 4); QUEUE_SIZE controls the jobs buffer (default twice the worker count). Server read/write/idle timeouts are configured. SIGINT/SIGTERM trigger graceful shutdown using a separate ten-second context, reusing the Day 13 lifecycle pattern.

### concurrencyapi/main_test.go

Tests cover input-order preservation, deadline cancellation, integer overflow, concurrent metrics access, successful HTTP quote calculation, unknown JSON-field rejection and metrics endpoint response.

## 10. Try the API

Start:

~~~bash
go run ./day-14/concurrencyapi
~~~

Health:

~~~bash
curl -i http://localhost:8080/healthz
~~~

Submit a batch:

~~~bash
curl -i -X POST http://localhost:8080/api/v1/quotes \
  -H "Content-Type: application/json" \
  -d '{"items":[{"sku":"KB-01","unit_price_paise":149900,"quantity":2,"processing_ms":400},{"sku":"MS-02","unit_price_paise":79900,"quantity":1,"processing_ms":100},{"sku":"HD-03","unit_price_paise":259900,"quantity":3,"processing_ms":250}]}'
~~~

Expected subtotals: 299800, 79900 and 779700 paise, returned in input order even though the second item finishes first.

Metrics:

~~~bash
curl -i http://localhost:8080/api/v1/metrics
~~~

## 11. Common mistakes

1. Starting one goroutine per input without a limit.
2. Sending on an unbuffered channel when no receiver can run.
3. Closing a channel from the receiver.
4. Sending after close.
5. Calling Wait before the goroutine count is established.
6. Sharing a map without synchronization.
7. Forgetting Unlock; defer it immediately after Lock.
8. Holding a lock while doing network I/O.
9. Using default in select and accidentally busy-looping.
10. Assuming results arrive in input order.
11. Ignoring cancellation while blocked on a channel send.
12. Treating a per-request worker count as a process-wide limit.

## 12. Exercises

1. Return the number of workers used for a batch.
2. Change the maximum batch size to 50 and test 50 versus 51 items.
3. Add a process-wide semaphore limiting all quote calculations to 16.
4. Add an active-calculations field to metrics.
5. Create an unsafe counter, run 100 goroutines against it, observe go test -race, then protect it with Mutex.
6. Test that every job is processed exactly once.
7. Preserve result order using the original index.
8. Cancel while the producer is blocked by a full jobs queue and assert prompt return.

### Suggested solutions

1. Return a BatchSummary containing WorkerCount=min(configured workers, item count) and the quotes.
2. Validate before starting goroutines; test both boundary values.
3. Use a shared buffered semaphore channel of capacity 16. Acquire in a select with ctx.Done and release with defer.
4. Increment/decrement active calculations under the Metrics mutex and include it in Snapshot.
5. Run the unsafe version with go test -race, then add Lock/Unlock around the increment.
6. Have the producer send and close once; count IDs in the test and assert no missing or duplicate IDs.
7. Send index with each outcome and assign ordered[index] in the collector.
8. Make producer sends and worker receives cancellation-aware; wait for workers and close results after WaitGroup completion.

## 13. Interview questions

1. What is a goroutine? — A lightweight concurrent execution unit managed by the Go runtime.
2. Goroutine versus OS thread? — The runtime multiplexes goroutines over OS threads.
3. What is G-M-P? — Goroutine, machine/thread and processor scheduling resource.
4. Concurrency versus parallelism? — Overlapping progress versus simultaneous execution.
5. Buffered versus unbuffered channel? — A buffer allows limited queued sends; unbuffered send and receive rendezvous.
6. What happens when a channel is closed? — No more sends; buffered values remain receivable.
7. Who closes a channel? — The sender/owner that knows no more values will be sent.
8. What is select? — Waits on multiple channel operations.
9. What does WaitGroup do? — Waits for goroutines to finish.
10. WaitGroup versus channel? — Completion tracking versus communication.
11. Mutex versus RWMutex? — Exclusive access versus multiple readers or one writer.
12. What is a data race? — Unsynchronized concurrent access to the same memory with at least one write.
13. What is deadlock? — Goroutines wait on events/resources that cannot occur.
14. What is backpressure? — Slowing/rejecting producers when downstream capacity is full.
15. Why bounded worker pools? — Protect CPU, memory, DB pools and partner APIs.
16. How preserve result order? — Carry original index and store by index.
17. Does cancel kill a goroutine? — No; goroutines must cooperate.
18. Why go test -race? — It detects data races exercised by tests.
19. Is a four-worker per-request pool a four-worker process limit? — No; concurrent requests multiply it.
20. What are fan-out and fan-in? — Distribute tasks to workers and combine their results.

## 14. Backend use cases

- Bulk API processing and record transformation.
- Image, document and OCR pipelines.
- Notification delivery with provider rate limits.
- Report generation from independent data sources.
- Inventory lookups with a global DB concurrency limit.
- Log ingestion with bounded buffers.
- Scheduled background jobs.

For your .NET background, compare a Go worker pool with a bounded Channel<T> plus a hosted background service. The concepts are similar: bounded queue, fixed consumers, cancellation and coordinated shutdown. Go channels are in-memory primitives, not durable message brokers.

## 15. Quality checks and production checklist

~~~bash
gofmt -w day-14/concurrencyapi/*.go
go test ./day-14/concurrencyapi
go test -race ./day-14/concurrencyapi
go vet ./day-14/concurrencyapi
go build -o ./bin/concurrency-api ./day-14/concurrencyapi
~~~

- [ ] Bound worker count and queue capacity.
- [ ] Estimate total concurrency across simultaneous requests.
- [ ] Define channel ownership and closing responsibility.
- [ ] Never send to a closed channel.
- [ ] Wait for goroutines before closing resources.
- [ ] Make blocking sends/receives cancellation-aware.
- [ ] Protect shared state with mutexes or atomics.
- [ ] Keep locks around small in-memory critical sections.
- [ ] Preserve task IDs/indexes when results can finish out of order.
- [ ] Run the race detector in CI.
- [ ] Measure queue depth, worker utilization, latency and rejected work.
- [ ] Use a durable broker if jobs must survive process restarts; in-memory channels are not durable.

## Next lesson

**Day 15 — Testing Go Backends:** table-driven tests, subtests, httptest, interfaces and fakes, integration tests, fixtures, coverage, race testing and testable architecture.
