# Day 16 — Docker, Configuration & Production Runtime Foundations

**Previous:** Day 15 — Testing Go Backend Applications  
**Next:** Day 17 — Observability: structured logs, metrics and tracing

## What you will build

A runnable Go HTTP service packaged as a multi-stage Docker image. It validates environment configuration, emits structured JSON logs, exposes liveness/readiness endpoints, runs as a non-root user and shuts down gracefully on SIGTERM.

Analogy:
- Dockerfile = recipe.
- Image = packaged template.
- Container = one running copy of that template.
- Registry = image warehouse.
- Compose = a local runbook for starting services.

## Learning goals

1. Understand images, containers, layers, registries, ports and networks.
2. Build a Go binary in a multi-stage Dockerfile.
3. Keep the Go compiler out of the runtime image.
4. Parse and validate environment variables into typed configuration.
5. Separate ordinary configuration from secrets.
6. Use JSON logs and request IDs.
7. Distinguish liveness from readiness.
8. Apply non-root, read-only filesystem and dropped-capability defaults.
9. Use Docker Compose, health checks, resource limits and log rotation.
10. Handle SIGTERM and graceful shutdown.

## Project structure

~~~text
go-backend-daily/
  go.mod
  .dockerignore
  day-16/
    README.md
    Dockerfile
    compose.yaml
    .env.example
    runtimeapi/
      main.go
      config.go
      logger.go
      handlers.go
      healthcheck.go
      config_test.go
      handler_test.go
      healthcheck_test.go
~~~

## Run locally

From the go-backend-daily module root:

~~~bash
gofmt -w day-16/runtimeapi/*.go
go test ./day-16/runtimeapi
go test -race ./day-16/runtimeapi
go vet ./day-16/runtimeapi
go run ./day-16/runtimeapi
~~~

PowerShell:

~~~powershell
$env:APP_ENV = "development"
$env:PORT = "8080"
$env:LOG_LEVEL = "debug"
go run ./day-16/runtimeapi
~~~

Endpoints:
- GET http://localhost:8080/healthz — liveness.
- GET http://localhost:8080/readyz — readiness.
- GET http://localhost:8080/api/v1/runtime — safe runtime metadata.

The runtime endpoint intentionally does not return environment variables, passwords, tokens or connection strings.

## Configuration

The same compiled binary should run in development, staging and production. Values are injected at startup and parsed once into a typed Config.

| Variable | Default | Meaning |
|---|---|---|
| APP_ENV | development | development, staging or production |
| PORT | 8080 | HTTP listening port |
| LOG_LEVEL | info | debug, info, warn or error |
| READ_HEADER_TIMEOUT | 5s | Maximum header-read time |
| READ_TIMEOUT | 10s | Maximum request-read time |
| WRITE_TIMEOUT | 15s | Maximum response-write time |
| IDLE_TIMEOUT | 60s | Keep-alive idle time |
| SHUTDOWN_TIMEOUT | 10s | Graceful shutdown deadline |

Invalid environment names, ports, log levels and durations fail fast. This is safer than silently starting with an unexpected value.

### Configuration is not a secret

Configuration: port, environment name, log level, timeouts.  
Secrets: database passwords, signing keys, API tokens and certificates.

Never commit a real .env file, bake secrets into Dockerfile ARG/ENV, print all environment variables, or expose them from a diagnostic endpoint. Use a secret manager or orchestrator secret mount in production. The checked-in .env.example contains only non-secret sample settings.

## Structured logs

The application uses Go's log/slog JSON handler and writes to stdout. A request log contains request ID, method, path, status and duration. It does not log query strings, headers or request bodies, which may contain sensitive data.

Example:

~~~json
{"level":"INFO","msg":"http_request","request_id":"...","method":"GET","path":"/healthz","status":200,"duration_ms":1}
~~~

Container runtimes collect stdout/stderr. Avoid relying on files inside an ephemeral container.

## Liveness vs readiness

- /healthz: can this process answer HTTP?
- /readyz: should this instance receive traffic?

A process can be alive but not ready. Do not normally make liveness depend on a database outage: a temporary database failure can otherwise restart every API instance and create a restart storm. Readiness can reflect critical dependency state. This sample has no external dependency, so readiness follows the server lifecycle.

The Docker HEALTHCHECK runs the binary's healthcheck subcommand. It uses a two-second HTTP timeout and requires HTTP 200.

## Multi-stage Dockerfile walkthrough

Builder stage:
- FROM golang:1.23-alpine AS builder supplies the compiler.
- WORKDIR creates a predictable build directory.
- COPY go.mod before source lets Docker reuse cached layers.
- Copy only day-16/runtimeapi into the build context.
- Run go test during image creation; a failing test fails the build.
- CGO_ENABLED=0 and GOOS=linux build a static Linux executable for this standard-library-only app.
- -trimpath removes local source paths.
- -ldflags="-s -w" removes symbol/debug tables.
- -X main.version injects a build version.

Runtime stage:
- alpine:3.22 is the smaller runtime base.
- Install CA certificates for future outbound TLS calls.
- Create UID/GID 10001 and run as that non-root user.
- Copy only the compiled binary from the builder.
- EXPOSE documents port 8080; it does not publish a host port.
- HEALTHCHECK checks the local liveness endpoint.
- ENTRYPOINT starts the server.

Pin production base images by digest and scan/update them regularly. Alpine is one option; a hardened or distroless image may be more suitable for a particular organization.

## Build and run Docker

From the module root:

~~~bash
docker build -f day-16/Dockerfile -t go-runtime-api:day16 .
docker run --rm -p 8080:8080 go-runtime-api:day16
curl http://localhost:8080/healthz
curl http://localhost:8080/readyz
curl http://localhost:8080/api/v1/runtime
~~~

Host port 9090 can map to container port 8080:

~~~bash
docker run --rm -p 9090:8080 -e APP_ENV=staging -e LOG_LEVEL=debug go-runtime-api:day16
~~~

## Docker Compose

~~~bash
docker compose --env-file day-16/.env.example -f day-16/compose.yaml up --build -d
docker compose --env-file day-16/.env.example -f day-16/compose.yaml ps
docker compose --env-file day-16/.env.example -f day-16/compose.yaml logs -f api
docker compose --env-file day-16/.env.example -f day-16/compose.yaml down
~~~

The sample Compose service:
- Binds the host port to 127.0.0.1.
- Uses restart: unless-stopped.
- Uses a read-only root filesystem with a small /tmp tmpfs.
- Drops Linux capabilities and enables no-new-privileges.
- Limits memory and process count.
- Allows a 15-second stop grace period.
- Rotates local container logs.

These resource limits are examples. Tune them using load tests and production telemetry.

## Container networking

Inside a container, localhost means that same container. It does not mean the host or another container. If a Compose database service is named db, the API connects to db:5432, not localhost:5432. From the host, use the published host port; between Compose services, use service DNS and the container port.

## Graceful shutdown

Docker sends SIGTERM first and SIGKILL after the stop grace period. The server:
1. Receives SIGINT/SIGTERM through signal.NotifyContext.
2. Marks readiness false.
3. Calls http.Server.Shutdown with a configured deadline.
4. Waits for Serve to return.
5. Exits cleanly.

Compose stop_grace_period should exceed the app's shutdown timeout with a small buffer. Background jobs must also be canceled and joined.

## Go source walkthrough

### config.go
1. Config stores typed values: int, slog.Level and time.Duration.
2. LoadConfig reads process environment; loadConfig accepts an injected getter so tests do not mutate the real environment.
3. APP_ENV is allow-listed.
4. PORT is parsed and checked from 1 to 65535.
5. LOG_LEVEL is mapped to slog levels.
6. time.ParseDuration accepts values such as 5s and 1m; zero/negative durations are rejected.
7. Only non-secret runtime settings are retained.

### logger.go
1. slog.NewJSONHandler writes structured JSON.
2. HandlerOptions.Level filters lower-severity events.
3. slog.New returns the logger used by the app.
4. Output goes to stdout for container collection.

### handlers.go
1. API receives config, logger, readiness state and startup time through dependency injection.
2. NewHandler registers method-aware standard-library routes.
3. /healthz is liveness; /readyz is readiness.
4. /api/v1/runtime returns safe metadata only.
5. Middleware creates a cryptographically random request ID and returns X-Request-ID.
6. statusRecorder captures the HTTP status for logs.
7. Logs contain method, path, status, request ID and duration, but not headers or bodies.

### healthcheck.go
1. checkHealth uses an HTTP client with a two-second timeout.
2. It closes the response body.
3. Any status other than 200 is unhealthy.
4. The URL builder is tested separately using httptest.NewServer.

### main.go
1. LoadConfig runs before listening; invalid configuration fails startup.
2. The healthcheck command is a second process mode for Docker.
3. http.Server has read-header, read, write and idle timeouts.
4. signal.NotifyContext receives shutdown signals.
5. net.Listen binds the port before readiness becomes true.
6. Serve runs in a goroutine and reports through a buffered channel.
7. On SIGTERM, readiness becomes false before Shutdown.
8. Shutdown waits for in-flight requests until the deadline.
9. http.ErrServerClosed is treated as normal shutdown.

### Tests
- config_test.go: defaults, overrides, invalid environment, port, log level and duration.
- handler_test.go: health/readiness transitions, safe metadata and request ID.
- healthcheck_test.go: healthy and unhealthy HTTP responses using temporary servers.

## Common mistakes

1. Running as root unnecessarily.
2. Copying the compiler into the runtime image.
3. Hardcoding environment-specific values.
4. Committing .env secrets.
5. Putting credentials in Dockerfile ARG/ENV.
6. Assuming EXPOSE publishes a port.
7. Using localhost to reach another container.
8. Writing logs only to ephemeral files.
9. Confusing liveness and readiness.
10. Ignoring SIGTERM.
11. Leaving resource limits unbounded.
12. Using mutable latest tags for production without pinning.
13. Assuming containers are automatically secure.
14. Using read-only root filesystem without explicitly providing needed writable paths.
15. Making app shutdown timeout longer than orchestrator grace period.

## Exercises and solutions

1. Map host 9090 to container 8080. Solution: publish 9090:8080 and call localhost:9090.
2. Enable debug logs. Solution: set LOG_LEVEL=debug.
3. Reject PORT=70000. Solution: parse and range-check 1–65535.
4. Why no compiler in final image? Solution: copy only the compiled binary from the builder stage.
5. EXPOSE vs ports? Solution: EXPOSE documents; ports publishes/maps.
6. Connect API to Compose PostgreSQL named db. Solution: use db:5432.
7. Add CACHE_ENABLED. Solution: parse with strconv.ParseBool and test default/invalid values.
8. Add /version. Solution: return build-injected version, never secrets.
9. Make readiness depend on a critical dependency. Solution: update readiness from dependency state, keeping liveness independent.
10. Set shutdown to 8 seconds. Solution: SHUTDOWN_TIMEOUT=8s and stop_grace_period slightly longer.

## Interview questions

1. Image vs container? Image is the template; container is a running instance.
2. Why multi-stage? Keeps compiler/tooling out of runtime.
3. What is layer caching? Reuses unchanged build steps.
4. What is .dockerignore? Excludes files from build context.
5. Does EXPOSE publish a port? No.
6. Host vs container port? Host-side mapping vs port inside container.
7. Why non-root? Limits the impact of process compromise.
8. Why read-only root? Reduces runtime filesystem mutation.
9. What are Linux capabilities? Fine-grained privileges.
10. What is a health check? A command that reports container health.
11. Liveness vs readiness? Process health vs permission to receive traffic.
12. What is graceful shutdown? Stop new work and finish in-flight work within a deadline.
13. Why environment variables? Same artifact can use different deployment settings.
14. Where should secrets live? Secret manager/orchestrator secret mechanism.
15. What does localhost mean inside a container? That container's network namespace.
16. Why JSON logs? Easier filtering and ingestion.
17. What does unless-stopped mean? Restart unless explicitly stopped.
18. Why memory/PID limits? Prevent unbounded host resource consumption.

## Backend use cases

- Package a Go REST API for consistent local/staging/production runtime.
- Deploy the same binary with environment-specific timeouts and ports.
- Run API and database as separate Compose services.
- Expose probes to an orchestrator.
- Collect stdout logs centrally.
- Enforce non-root and read-only defaults.
- Gracefully stop APIs during rolling deployments.
- Keep credentials outside source and image.

## Day 16 checklist

- [ ] Explain image vs container.
- [ ] Explain both Dockerfile stages.
- [ ] Build and run the image.
- [ ] Change PORT and LOG_LEVEL.
- [ ] Run Compose and inspect logs/health.
- [ ] Explain EXPOSE vs published ports.
- [ ] Explain container localhost and service DNS.
- [ ] Explain liveness vs readiness.
- [ ] Explain non-root, read-only filesystem and dropped capabilities.
- [ ] Stop the container and explain graceful shutdown.
- [ ] Keep secrets out of Git and image layers.

**Next:** Day 17 — Observability: structured logs, metrics and tracing.
