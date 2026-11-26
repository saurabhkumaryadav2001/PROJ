//go:build integration && pgx

package main

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"os"
	"strings"
	"testing"
	"testing/fstest"
	"time"

	_ "github.com/jackc/pgx/v5/stdlib"
)

func TestPostgresMigrationRunnerIsIdempotentAndDetectsDrift(t *testing.T) {
	dsn := os.Getenv("TEST_DATABASE_URL")
	if dsn == "" {
		t.Skip("set TEST_DATABASE_URL to a disposable PostgreSQL test database")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	db, err := sql.Open("pgx", dsn)
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	if err := db.PingContext(ctx); err != nil {
		t.Fatal(err)
	}

	schema := fmt.Sprintf("migration_test_%d", time.Now().UnixNano())
	if _, err := db.ExecContext(ctx, `CREATE SCHEMA "`+schema+`"`); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		_, _ = db.ExecContext(context.Background(), `DROP SCHEMA IF EXISTS "`+schema+`" CASCADE`)
	})

	files := fstest.MapFS{
		"migrations/001_create_probe.up.sql": {Data: []byte("CREATE TABLE probe (id BIGINT PRIMARY KEY);")},
		"migrations/002_add_name.up.sql": {Data: []byte("ALTER TABLE probe ADD COLUMN name TEXT NOT NULL DEFAULT 'ok';")},
	}
	runner := Runner{DB: db, FS: files, Dir: "migrations", Schema: schema, LockID: 2026112602}
	if err := runner.Up(ctx); err != nil {
		t.Fatalf("first Up: %v", err)
	}
	if err := runner.Up(ctx); err != nil {
		t.Fatalf("second Up should be idempotent: %v", err)
	}
	status, err := runner.Status(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if len(status) != 2 {
		t.Fatalf("status has %d rows, want 2", len(status))
	}

	changed := fstest.MapFS{
		"migrations/001_create_probe.up.sql": {Data: []byte("CREATE TABLE probe (id BIGINT PRIMARY KEY, changed BOOLEAN);")},
		"migrations/002_add_name.up.sql": {Data: []byte("ALTER TABLE probe ADD COLUMN name TEXT NOT NULL DEFAULT 'ok';")},
	}
	driftRunner := Runner{DB: db, FS: changed, Dir: "migrations", Schema: schema, LockID: 2026112602}
	err = driftRunner.Up(ctx)
	if !errors.Is(err, ErrChecksumMismatch) && !strings.Contains(fmt.Sprint(err), ErrChecksumMismatch.Error()) {
		t.Fatalf("drift error = %v, want checksum mismatch", err)
	}
}
