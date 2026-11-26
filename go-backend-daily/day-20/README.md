# Day 20 — Database Migrations, Integration Testing & Production Hardening

Previous: [Day 19 — PostgreSQL-backed Authentication](../day-19/README.md)

Day 20 adds a versioned migration runner to the Go/PostgreSQL curriculum. It teaches why schema changes must be repeatable, auditable, ordered, serialized and tested against a real database.

## Learning outcomes

- Explain schema drift and why CREATE TABLE IF NOT EXISTS alone is not a migration strategy.
- Use immutable, ordered SQL migration files.
- Embed migration SQL in a Go binary with embed.FS.
- Record applied versions and SHA-256 checksums in schema_migrations.
- Make migration execution idempotent and detect edited or deleted historical migrations.
- Serialize concurrent migration runners with a PostgreSQL advisory lock.
- Apply each schema change and its ledger row in one transaction.
- Run unit tests and an opt-in real-PostgreSQL integration test.
- Understand expand-and-contract deployment, backup, rollback and pool-sizing practices.

## Analogy

A database schema is like a building. A migration is a numbered, approved change to its blueprint. The migration ledger is the building's maintenance log. Never rewrite an old entry after it has been applied; add a new numbered change instead.

## Project structure

    go-backend-daily/
    ├── go.mod
    └── day-20/
        ├── README.md
        └── migrationapi/
            ├── main.go
            ├── db.go
            ├── migrations.go
            ├── driver_pgx.go
            ├── migrations_test.go
            ├── postgres_integration_test.go
            └── migrations/
                ├── 001_create_widgets.up.sql
                ├── 002_add_widget_active.up.sql
                └── 003_index_widget_name.up.sql

## How the runner works

1. embed.FS packages the .up.sql files into the executable.
2. Discover validates file names, extracts numeric versions, rejects duplicates and empty SQL, then sorts ascending.
3. Runner validates the schema identifier, creates the schema if necessary, obtains one dedicated sql.Conn and sets search_path on that connection.
4. A PostgreSQL session advisory lock prevents two migration processes from running together.
5. schema_migrations records version, name, checksum and applied_at.
6. Before applying, the runner compares the current SHA-256 checksum with the stored checksum. Editing or deleting an applied migration fails loudly.
7. Each SQL file runs inside a transaction. The ledger row is inserted in that same transaction.
8. The advisory lock is released on the same connection. PostgreSQL releases a session lock if the connection dies.

### Why a dedicated sql.Conn?

sql.DB is a pool. A session-level advisory lock belongs to one PostgreSQL connection. If the lock is acquired through db.ExecContext and migration SQL runs through another pooled connection, the lock does not protect the work. DB.Conn pins one connection for search_path, lock, ledger queries and migration transactions.

### Why checksums?

If migration 001 was applied in production and its source file is later edited, a fresh database could get a different schema. SHA-256 detects that drift. Do not modify applied migration files; add migration 004 instead.

### One statement per migration file

This teaching runner sends each file as one ExecContext call. Driver protocol behavior for multiple statements can vary. Keep each file to one DDL statement; split complex changes into multiple versions. For complex SQL or non-transactional DDL, use a mature migration library.

## Commands

The existing module already declares github.com/jackc/pgx/v5 v5.7.2.

Set a disposable local PostgreSQL database:

    export DATABASE_URL='postgres://postgres:postgres@localhost:5432/go_backend?sslmode=disable'
    export MIGRATION_SCHEMA='public'
    go mod download github.com/jackc/pgx/v5

Apply pending migrations:

    go run -tags pgx ./day-20/migrationapi up

Show applied versions:

    go run -tags pgx ./day-20/migrationapi status

PowerShell:

    $env:DATABASE_URL = "postgres://postgres:postgres@localhost:5432/go_backend?sslmode=disable"
    $env:MIGRATION_SCHEMA = "public"
    go run -tags pgx ./day-20/migrationapi up
    go run -tags pgx ./day-20/migrationapi status

## Tests

Unit tests use testing/fstest.MapFS and need no live database:

    go test ./day-20/migrationapi
    go test -race ./day-20/migrationapi
    go vet ./day-20/migrationapi
    go test -cover ./day-20/migrationapi

Optional real PostgreSQL test (use a disposable test database; it creates and drops a uniquely named schema):

    export TEST_DATABASE_URL='postgres://postgres:postgres@localhost:5432/go_backend_test?sslmode=disable'
    go test -tags='integration pgx' ./day-20/migrationapi -run TestPostgresMigrationRunnerIsIdempotentAndDetectsDrift -count=1 -v

The integration test checks first-run application, second-run idempotency, status output and checksum drift. Never point TEST_DATABASE_URL at production.

## Code walkthrough

### migrations.go

- go:embed includes SQL files at build time.
- Discover uses fs.Glob, validates NNN_name.up.sql, parses numeric versions, rejects duplicates and calculates SHA-256.
- Runner.Up pins one connection, sets a validated/quoted search_path, acquires an advisory lock and creates the ledger.
- It detects missing or modified historical migration files before applying new versions.
- applyOne starts a transaction, executes SQL, inserts the ledger row and commits.
- defer tx.Rollback safely handles early returns; after Commit it is harmless.
- readApplied closes rows and checks rows.Err.
- Status reads the ledger without applying changes.

### db.go

sql.Open creates a pool manager; PingContext proves connectivity. Pool limits and connection lifetimes are validated. MaxOpenConns multiplied by the number of API replicas must fit PostgreSQL's total connection budget.

### driver_pgx.go

The pgx build tag registers the driver only for real PostgreSQL commands. Unit tests compile without downloading or requiring a database driver.

### postgres_integration_test.go

Build tags integration and pgx keep the test opt-in. It uses a uniquely named schema, verifies idempotency and checksum drift, then drops the test schema.

## Production migration strategy

Prefer a separate one-shot release job to run migrations before deploying new API replicas. Advisory locking is defense in depth, not a reason to run migrations from every API process at startup.

A safe expand-and-contract rollout:

1. Expand: add a nullable column, table or index that old code tolerates.
2. Deploy code that can read/write both old and new shapes.
3. Backfill data in bounded, resumable batches.
4. Switch reads to the new field and verify data and metrics.
5. Contract: remove old columns only after old application versions are gone.

Avoid long table locks and full-table rewrites during peak traffic. CREATE INDEX CONCURRENTLY cannot run inside a normal transaction block; this teaching runner intentionally does not support that special case. Use a mature migration tool or a carefully designed non-transactional migration path.

### Rollback is not always down

Dropping a new column may destroy data written by the new application. Prefer a forward-fix migration or a tested restore plan. A migration should be reversible only when the reverse operation is genuinely safe.

## Production checklist

- Keep credentials in a secret manager; never commit .env or real secrets.
- Use TLS for remote PostgreSQL.
- Run migrations as a release job with a least-privilege migration account.
- Back up and test restores before risky DDL or data migrations.
- Monitor migration duration, lock waits, connection count, replication lag and disk space.
- Keep runtime DB users separate from schema-owner/migration users.
- Use statement and lock timeouts appropriate to the deployment window.
- Test on production-like data volumes.
- Make backfills resumable and batch them.
- Keep applied migration files immutable.
- Count all replicas when sizing the connection pool.
- Never run destructive integration tests against production.

## Exercises and solutions

1. Add migration 004 for description TEXT. Create 004_add_widget_description.up.sql containing ALTER TABLE widgets ADD COLUMN description TEXT;. Run up; status should show four versions.
2. Prove idempotency. Run up twice. The second run applies zero new versions; the ledger still has three starter rows.
3. Simulate checksum drift. Change an already-applied SQL file in a test fstest.MapFS; expect ErrChecksumMismatch.
4. Detect a deleted historical file. Apply versions 001 and 002, then supply only 002; the runner must report that applied version 001 is missing.
5. Add statement and lock timeouts. Configure PostgreSQL lock_timeout and statement_timeout for the deployment window.
6. Add down migrations only for operations that cannot destroy important data. Prefer forward fixes when uncertain.
7. Prevent startup stampede by running migrations in CI/CD as a one-shot job and gating app deployment on success.

## Interview questions

1. What is a migration? A versioned, repeatable schema/data change.
2. What is schema drift? Different environments have different schema definitions or histories.
3. Why maintain a ledger? To record which versions have been applied and when.
4. Why use checksums? To detect edits to historical migration files.
5. What does idempotent mean? Re-running the command does not reapply completed versions.
6. Why use advisory locks? To serialize cooperating migration processes.
7. Why pin one sql.Conn? Session locks and search_path belong to one PostgreSQL session.
8. Why insert the ledger row in the same transaction? Schema and history record commit or roll back together.
9. What is expand-and-contract? A staged backward-compatible schema rollout.
10. Why can down migrations be dangerous? They can destroy data written after the forward migration.
11. Why is CREATE INDEX CONCURRENTLY special? PostgreSQL does not allow it inside a normal transaction block.
12. Why use a one-shot migration job? It separates schema changes from API replica startup and runtime privileges.

## Real backend use cases

- HRMS: add employee department_id, backfill existing rows, then switch reads.
- Fleet/transport: add breakdown severity and status history without blocking active dispatch.
- E-commerce: add payment state while old and new checkout API versions overlap.
- Authentication: add refresh-session device metadata without invalidating existing sessions.
- SaaS: apply tenant schema/configuration changes while tracking progress.

## Test results and limitations

The standard-library unit suite uses a fake database/sql driver and tests ordering, duplicates, schema validation, idempotency, checksum drift and failed-migration bookkeeping. Live PostgreSQL behavior still requires the opt-in integration test.

## Next lesson

Day 21 — Redis caching: cache-aside, TTL, invalidation, stampede protection, Redis data structures, rate limiting and cache observability.
