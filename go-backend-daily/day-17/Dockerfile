FROM golang:1.23-alpine AS builder
WORKDIR /src
COPY go.mod ./
COPY day-17/observabilityapi ./day-17/observabilityapi
RUN CGO_ENABLED=0 GOOS=linux go test ./day-17/observabilityapi
ARG VERSION=dev
RUN CGO_ENABLED=0 GOOS=linux go build -trimpath -ldflags="-s -w -X main.version=${VERSION}" -o /out/server ./day-17/observabilityapi

FROM alpine:3.22
RUN addgroup -S -g 10001 app && adduser -S -D -H -u 10001 -G app app
WORKDIR /app
COPY --from=builder --chown=10001:10001 /out/server /app/server
USER 10001:10001
ENV PORT=8080 APP_VERSION=dev
EXPOSE 8080
ENTRYPOINT ["/app/server"]
