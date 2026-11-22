# Stage 1: compile and test the Go application.
FROM golang:1.23-alpine AS builder

WORKDIR /src

# Copy module metadata first so Docker can cache dependency resolution.
COPY go.mod ./
COPY day-16/runtimeapi ./day-16/runtimeapi

# Fail the image build if the application tests fail.
RUN CGO_ENABLED=0 GOOS=linux go test ./day-16/runtimeapi

# Build a small static Linux binary. VERSION can be supplied at build time.
ARG VERSION=dev
RUN CGO_ENABLED=0 GOOS=linux go build \
    -trimpath \
    -ldflags="-s -w -X main.version=${VERSION}" \
    -o /out/server \
    ./day-16/runtimeapi

# Stage 2: run only the compiled binary in a small runtime image.
FROM alpine:3.22

RUN apk add --no-cache ca-certificates \
    && addgroup -S -g 10001 app \
    && adduser -S -D -H -u 10001 -G app app

WORKDIR /app

COPY --from=builder --chown=10001:10001 /out/server /app/server

USER 10001:10001

ENV APP_ENV=production \
    PORT=8080 \
    LOG_LEVEL=info

EXPOSE 8080

# The application includes a tiny healthcheck subcommand; curl is not needed.
HEALTHCHECK --interval=30s --timeout=3s --start-period=5s --retries=3 \
    CMD ["/app/server", "healthcheck"]

ENTRYPOINT ["/app/server"]
