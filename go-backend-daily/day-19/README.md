# Day 19 — PostgreSQL-backed Authentication in Go

Previous: [Day 18 — Authentication & Authorization](../day-18/README.md)

Day 19 replaces Day 18's process-local MemoryStore with a persistent PostgreSQL repository using database/sql and the existing pgx v5.7.2 dependency.

## What is included
- Context-aware Store interface and PostgresStore implementation.
- Parameterized SQL, unique-email error mapping and connection-pool configuration.
- Atomic refresh-token rotation with SELECT FOR UPDATE.
- Refresh-token family reuse detection and logout revocation.
- PostgreSQL schema, Go API, tests, Dockerfile and Compose configuration.
- PostgreSQL repository excerpt: [postgres_store.go](authapi/postgres_store.go).
- Database schema: [001_auth_tables.sql](migrations/001_auth_tables.sql).
- The complete runnable modular project and tests are supplied in the accompanying Day 19 code ZIP.

The repository currently contains the lesson, the core PostgreSQL repository adapter and the schema migration. The complete modular API, service, middleware, tests, Dockerfile and Compose project are included in the attached code ZIP.

## Run
1. Apply the schema: `psql "$DATABASE_URL" -f day-19/migrations/001_auth_tables.sql`
2. Download the existing driver dependency: `go mod download github.com/jackc/pgx/v5`
3. Set `DATABASE_URL` and a unique `JWT_SECRET` of at least 32 bytes.
4. Extract the complete code ZIP into the existing `go-backend-daily` module root, then run `go test ./day-19/authapi`.
5. Run against PostgreSQL with the existing pgx dependency: `go run -tags pgx ./day-19/authapi`.

The pgx build tag keeps the default unit-test suite buildable without downloading the external driver. The real PostgreSQL integration test requires `-tags "pgx integration"` and `TEST_DATABASE_URL`.

**Security note:** the sample continues Day 18's PBKDF2 teaching hasher. Prefer a vetted Argon2id library and security review for a new production system. Never commit real secrets.
