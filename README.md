# RecallAPI 🧠
Private searchable memory service. Store notes, ideas and snippets behind a Go REST API; search full content, filter tags and set expiry.

## Features
- Register/login using bcrypt + signed JWT
- User-isolated memory CRUD
- PostgreSQL full-text search (GIN) + tags + pagination
- Optional expiry timestamps
- Health/readiness probes, request IDs, structured JSON logs
- Graceful shutdown, server timeouts, Docker Compose, embedded migration, GitHub Actions CI

## Quick start
Requirements: Go 1.23+, Docker.
```bash
export JWT_SECRET="$(openssl rand -hex 32)"
docker compose up --build
```
API: localhost:8080; liveness /healthz; readiness /readyz.

## Routes
- POST /v1/auth/register — email + password (12-72 chars)
- POST /v1/auth/login
- POST /v1/memories — create
- GET /v1/memories?q=go&tag=learning&limit=20&offset=0 — search/list
- GET /v1/memories/{id}
- PUT /v1/memories/{id}
- DELETE /v1/memories/{id}

Protected routes require Authorization: Bearer TOKEN.
```bash
TOKEN=$(curl -s localhost:8080/v1/auth/register -H 'Content-Type: application/json' -d '{"email":"you@example.com","password":"correct-horse-battery"}' | jq -r .access_token)
curl -s localhost:8080/v1/memories -H "Authorization: Bearer $TOKEN" -H 'Content-Type: application/json' -d '{"title":"Go notes","content":"Contexts cancel work","tags":["go","learning"]}'
curl 'localhost:8080/v1/memories?q=contexts' -H "Authorization: Bearer $TOKEN"
```

## Configuration
HTTP_ADDR (default :8080), DATABASE_URL (local PostgreSQL default), JWT_SECRET (set to >=32 random bytes in production).

## Production hardening before real traffic
This is a production-oriented portfolio MVP, not yet certified production-ready. Deploy behind TLS; store secrets in a manager; add gateway/Redis rate limiting (not currently implemented), backups, monitoring, alerting, migration rollout controls, and load/security testing. The default JWT secret is development-only.
