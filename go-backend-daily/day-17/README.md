# Day 17 — Observability: Logs, Metrics and Tracing

Previous: Day 16 — Docker, Configuration & Production Runtime
Next: Day 18 — Authentication and Authorization (JWT, middleware and RBAC)

## What you will build

A runnable Go HTTP API with:
- Structured JSON logs using the standard-library log/slog package.
- A concurrency-safe Prometheus text-format metrics endpoint.
- Request counters, request-duration histograms and an in-flight gauge.
- Bounded metric labels based on Go ServeMux route patterns, not raw URLs or IDs.
- W3C Trace Context (traceparent) parsing and propagation.
- Server spans and child-span lifecycle logs for a sample order lookup.
- Prometheus, Grafana and a provisioned dashboard through Docker Compose.
- Graceful shutdown and automated tests, including race and fuzz tests.

Scope note: the Go application uses only the standard library. The trace example demonstrates W3C trace-context propagation and span boundaries, with span events written to structured logs. It is a small teaching tracer, not a complete OpenTelemetry SDK or OTLP exporter. In production, use OpenTelemetry Go instrumentation/SDK and export OTLP spans to an OpenTelemetry Collector or tracing backend such as Tempo or Jaeger.

## Analogy

Imagine an airport:
- Logs are the incident diary: what happened, with useful context.
- Metrics are the control-room gauges: how many flights, delays and active runways right now.
- Traces are one passenger's journey across check-in, security, gate and aircraft, showing where time was spent.

They answer different questions. A log is an event. A metric is an aggregate measurement over time. A trace is a connected path of work across components.

## Why observability matters

Without observability, a production report such as "the checkout API is slow" is difficult to diagnose. With all three signals:
1. Metrics show p95 latency increased at 10:42.
2. Logs filtered by trace ID show the affected request and error.
3. A trace across API, inventory and payment spans identifies the slow dependency.

Monitoring asks whether known conditions are healthy. Observability helps investigate conditions that were not anticipated in advance.

## Project structure

~~~text
go-backend-daily/
  go.mod
  day-17/
    README.md
    Dockerfile
    compose.yaml
    prometheus/prometheus.yml
    grafana/
      provisioning/datasources/prometheus.yml
      provisioning/dashboards/dashboards.yml
      dashboards/go-api-overview.json
    observabilityapi/
      main.go
      config.go
      logger.go
      metrics.go
      trace.go
      middleware.go
      handlers.go
      observability_test.go
~~~

## Run locally

From the go-backend-daily module root:

~~~bash
gofmt -w day-17/observabilityapi/*.go
go test ./day-17/observabilityapi
go test -race ./day-17/observabilityapi
go vet ./day-17/observabilityapi
go test -cover ./day-17/observabilityapi
go run ./day-17/observabilityapi
~~~

PowerShell:

~~~powershell
$env:PORT = "8080"
$env:APP_VERSION = "local"
go run ./day-17/observabilityapi
~~~

Endpoints:
- GET /healthz — liveness.
- GET /readyz — readiness.
- GET /metrics — Prometheus metrics.
- GET /api/v1/runtime — safe build/runtime metadata.
- GET /api/v1/orders/{id} — sample order endpoint with a child span.

Try:

~~~bash
curl -i http://localhost:8080/healthz
curl -i http://localhost:8080/readyz
curl -i -H 'traceparent: 00-4bf92f3577b34da6a3ce929d0e0e4736-00f067aa0ba902b7-01' http://localhost:8080/api/v1/orders/42
curl http://localhost:8080/metrics
~~~

The order endpoint returns a sample response; it does not connect to a real order database.

## 1. Structured logging

A plain message such as "request failed" is hard to search. A structured JSON event can include stable fields:

~~~json
{"time":"2026-11-23T10:00:00Z","level":"INFO","msg":"http_request","trace_id":"4bf...","span_id":"00f...","method":"GET","route":"/api/v1/orders/{id}","status":200,"duration_ms":4}
~~~

The sample uses slog.NewJSONHandler. Each request log includes trace ID, server span ID, HTTP method, route template, status code and elapsed milliseconds.

It intentionally does not log request bodies, cookies, authorization headers, query strings or customer identifiers. Those can contain credentials or personal data.

### Why route templates matter

Do not use the raw URL as a metric label. /users/1, /users/2, /users/3 creates a new time series for every ID. This is high cardinality and can overload a metrics backend. Go's method-aware ServeMux provides route patterns such as GET /api/v1/orders/{id}. The middleware records the route template rather than the raw path.

## 2. Metrics: counter, gauge and histogram

Counter: only increases (until process restart). Use it for totals such as requests, errors, jobs processed or retries.

~~~promql
sum(rate(go_http_requests_total[5m]))
~~~

Gauge: can increase and decrease. Use it for current values such as active requests, queue depth or open connections.

~~~promql
go_http_requests_in_flight
~~~

Histogram: records observations in buckets plus a sum and count. Use it for request duration and payload size.

~~~promql
histogram_quantile(0.95, sum by (le) (rate(go_http_request_duration_seconds_bucket[5m])))
~~~

This estimates p95 request latency. A histogram is not an average; it allows percentile estimation from bucket distributions.

### Metrics implementation walkthrough

1. Metrics owns maps for request counters and duration histograms plus an in-flight gauge.
2. A sync.Mutex protects shared maps and the gauge because HTTP handlers run concurrently.
3. Begin increments in-flight requests.
4. End decrements the gauge in deferred cleanup.
5. Record increments the counter and observes elapsed seconds.
6. Each histogram bucket is cumulative.
7. WritePrometheus copies a consistent snapshot under the lock, then releases the lock before writing to the network. A slow scraper therefore does not block request recording.
8. The output uses Prometheus text exposition format with HELP and TYPE metadata.

This small registry is for teaching. For production applications with many metrics, use the official Prometheus Go client library rather than maintaining a custom registry.

## 3. Tracing and W3C Trace Context

A distributed trace represents one logical operation across services. Each trace has a trace ID; each operation has a span ID. A child span keeps the trace ID but receives a new span ID and records its parent span ID.

The W3C traceparent header has this version-00 form:

~~~text
00-{32 hex trace ID}-{16 hex parent span ID}-{2 hex trace flags}
~~~

The middleware:
1. Reads the incoming traceparent header.
2. Validates version, lengths, hexadecimal characters and non-zero IDs.
3. Continues the incoming trace ID if valid; otherwise starts a new trace.
4. Creates a new server span ID.
5. Adds trace context to the request context.
6. Returns X-Trace-ID so a caller can correlate its request with logs.

StartSpan creates a child span for orders.lookup. InjectTraceParent demonstrates propagation on an outgoing HTTP request:

~~~go
request, err := http.NewRequestWithContext(ctx, http.MethodGet, downstreamURL, nil)
if err != nil { return err }
InjectTraceParent(ctx, request.Header)
response, err := http.DefaultClient.Do(request)
~~~

The teaching tracer logs span_started and span_finished; it does not export spans to a trace database. Production tracing should use OpenTelemetry, instrument HTTP clients/servers and database calls, configure sampling, and export OTLP to a collector/backend.

## 4. Middleware: connect the signals

ObservabilityMiddleware:
1. Extracts or creates trace context.
2. Adds trace ID to the response header.
3. Increments in-flight requests.
4. Wraps the response writer to capture status.
5. Calls the next handler.
6. Decrements in-flight count in a defer.
7. Reads the matched route pattern, not the raw path.
8. Records request counter and latency histogram.
9. Emits a structured request log with trace/span IDs.

The sample response writer supports ordinary JSON APIs and Unwrap. If production handlers use WebSockets, HTTP streaming, HTTP/2 push or custom ResponseWriter interfaces, preserve the interfaces they need or use http.ResponseController.

## 5. Prometheus and Grafana stack

The included Compose file starts:
- API at 127.0.0.1:8080.
- Prometheus at 127.0.0.1:9090.
- Grafana at 127.0.0.1:3000.

Set a local Grafana password before starting.

Bash:

~~~bash
export GRAFANA_ADMIN_PASSWORD='choose-a-local-password'
docker compose -f day-17/compose.yaml up --build -d
~~~

PowerShell:

~~~powershell
$env:GRAFANA_ADMIN_PASSWORD = "choose-a-local-password"
docker compose -f day-17/compose.yaml up --build -d
~~~

Then:
- API: http://localhost:8080/healthz
- Prometheus: http://localhost:9090/targets
- Grafana: http://localhost:3000 (user admin, password from environment variable)

Prometheus scrapes api:8080/metrics using Compose service DNS. Grafana's Prometheus data source and sample dashboard are provisioned automatically.

The dashboard includes request rate, p95 latency, 5xx error rate and in-flight requests. This is a local learning stack; before production, pin images by digest, configure TLS/authentication, persistent storage, access control, retention, alerting and secrets management.

Stop:

~~~bash
docker compose -f day-17/compose.yaml down
~~~

## 6. PromQL queries

Request rate:

~~~promql
sum by (route) (rate(go_http_requests_total[5m]))
~~~

5xx error rate:

~~~promql
sum(rate(go_http_requests_total{status=~"5.."}[5m]))
~~~

Error ratio:

~~~promql
sum(rate(go_http_requests_total{status=~"5.."}[5m]))
/
sum(rate(go_http_requests_total[5m]))
~~~

p95 latency:

~~~promql
histogram_quantile(0.95, sum by (le) (rate(go_http_request_duration_seconds_bucket[5m])))
~~~

In-flight requests:

~~~promql
go_http_requests_in_flight
~~~

## 7. Configuration

The sample reads PORT, APP_VERSION and HTTP/shutdown timeouts once at startup. Invalid ports or non-positive durations fail startup. Do not expose environment variables from a diagnostic endpoint and never put credentials in source control or dashboard labels.

## 8. Quality checks

~~~bash
go test ./day-17/observabilityapi
go test -race ./day-17/observabilityapi
go vet ./day-17/observabilityapi
go test -cover ./day-17/observabilityapi
go test -fuzz=FuzzParseTraceParent -fuzztime=10s ./day-17/observabilityapi
go test -run '^$' -bench=BenchmarkHealthEndpointWithObservability -benchmem ./day-17/observabilityapi
go build ./day-17/observabilityapi
~~~

## Exercises and suggested solutions

1. Add an application error counter. Increment a bounded counter for 5xx; do not label it with order ID or error text.
2. Add POST /api/v1/orders. Register a fixed method-aware pattern and test success and invalid JSON with httptest.
3. Why should raw URL paths not be metric labels? IDs and arbitrary path values create unbounded cardinality. Use route templates.
4. Counter vs gauge? Counter accumulates; gauge can go up or down.
5. What is p95? A latency threshold at or below which 95% of observed requests fall, estimated from histogram buckets.
6. Continue a trace across two services. Extract inbound traceparent, create a server span, then call InjectTraceParent on the downstream request.
7. Why not log Authorization headers? They contain credentials and may leak tokens into log storage.
8. How would you find the slow endpoint? Inspect p95 histogram by route, then use trace IDs to locate slow spans.
9. What is high cardinality? Too many distinct label combinations, such as a metric label containing each user's ID.
10. How would you replace the teaching tracer in production? Use OpenTelemetry Go SDK, propagate W3C context, export OTLP to a collector, and configure sampling/retention.

## Interview questions

1. What is observability? Inferring internal system behavior from logs, metrics and traces.
2. Logging vs metrics? Logs are individual events; metrics are numeric aggregates over time.
3. What is a trace? A connected representation of work across components.
4. What is a span? One timed operation inside a trace.
5. Trace ID vs span ID? Trace ID groups the whole request; span ID identifies one operation.
6. What is context propagation? Carrying trace context across process/service boundaries.
7. What is W3C Trace Context? A standard header format, commonly traceparent, for propagating trace identity.
8. Counter vs gauge? Monotonically increasing total vs current value.
9. What is a histogram? Bucketed observations plus sum and count.
10. Why use p95 rather than only average? Averages can hide slow tail requests.
11. What is Prometheus scraping? Prometheus periodically requests a metrics endpoint and stores samples.
12. What is label cardinality? Number of unique label combinations/time series.
13. Why avoid user IDs in labels? They cause unbounded time-series growth and may expose sensitive identifiers.
14. What is structured logging? Logs represented as fields in a consistent machine-readable format.
15. What is OpenTelemetry? Vendor-neutral APIs/SDKs and conventions for traces, metrics and logs.
16. What is an OTLP collector? A component that receives, processes and exports telemetry.
17. What is sampling? Deciding which traces to record/export to manage overhead and volume.
18. How do logs and traces correlate? Include trace ID/span ID in log records.

## Backend use cases

- Diagnose slow REST endpoints in a Go microservice fleet.
- Alert when 5xx rate or p95 latency crosses a service-level objective.
- Follow a request through API, SQL, Redis and message broker calls.
- Correlate production error logs with a distributed trace.
- Measure queue depth, active workers, retries and database pool usage.
- Build dashboards for deployments and incident response.

## Common mistakes

- Logging entire request/response bodies without redaction.
- Using a unique user/order/request ID as a Prometheus label.
- Treating a counter as a gauge.
- Calling an average a p95 latency.
- Generating a new trace ID at every service instead of continuing incoming context.
- Forgetting to create a new span ID for each operation.
- Assuming a log-only span demo is a full OpenTelemetry exporter.
- Exposing /metrics publicly without access/network controls.
- Holding a global metrics lock while writing to a slow network client.
- Ignoring metric naming, units and bounded labels.

## Day 17 checklist

- [ ] Explain logs, metrics and traces with an example.
- [ ] Read a JSON slog event.
- [ ] Explain counter, gauge and histogram.
- [ ] Run /metrics and understand its exposition.
- [ ] Explain why route templates avoid high cardinality.
- [ ] Read and validate a W3C traceparent.
- [ ] Explain trace ID, span ID and parent span ID.
- [ ] Propagate trace context to a downstream HTTP request.
- [ ] Run Prometheus and Grafana with Compose.
- [ ] Query request rate, 5xx rate and p95 latency.
- [ ] Explain why production tracing should use OpenTelemetry/OTLP.

Next: Day 18 — Authentication and Authorization: JWT, middleware, password hashing and role-based access control.
