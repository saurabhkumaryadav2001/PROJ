FROM golang:1.23-alpine AS builder
WORKDIR /src
COPY go.mod ./
COPY day-18/authapi ./day-18/authapi
RUN CGO_ENABLED=0 GOOS=linux go test ./day-18/authapi
RUN CGO_ENABLED=0 GOOS=linux go build -trimpath -ldflags="-s -w" -o /out/auth-api ./day-18/authapi

FROM alpine:3.22
RUN addgroup -S -g 10001 app && adduser -S -D -H -u 10001 -G app app
WORKDIR /app
COPY --from=builder --chown=10001:10001 /out/auth-api /app/auth-api
USER 10001:10001
EXPOSE 8080
ENTRYPOINT ["/app/auth-api"]
