package main

import (
	"context"
	"database/sql"
	"database/sql/driver"
	"errors"
	"fmt"
	"io"
	"sort"
	"strings"
	"sync"
	"testing"
	"testing/fstest"
	"time"
)

func TestDiscoverSortsMigrationsAndCalculatesChecksum(t *testing.T) {
	files := fstest.MapFS{
		"migrations/002_add_flag.up.sql": {Data: []byte("ALTER TABLE items ADD COLUMN active BOOLEAN NOT NULL DEFAULT TRUE;")},
		"migrations/001_create_items.up.sql": {Data: []byte("CREATE TABLE items (id BIGINT PRIMARY KEY);")},
	}
	got, err := Discover(files, "migrations")
	if err != nil { t.Fatal(err) }
	if len(got) != 2 { t.Fatalf("got %d migrations, want 2", len(got)) }
	if got[0].Version != 1 || got[1].Version != 2 { t.Fatalf("versions not sorted: %+v", got) }
	if len(got[0].Checksum) != 64 { t.Fatalf("checksum length = %d, want 64", len(got[0].Checksum)) }
}

func TestDiscoverRejectsDuplicateVersions(t *testing.T) {
	files := fstest.MapFS{
		"migrations/001_create_a.up.sql": {Data: []byte("SELECT 1;")},
		"migrations/001_create_b.up.sql": {Data: []byte("SELECT 2;")},
	}
	_, err := Discover(files, "migrations")
	if !errors.Is(err, ErrInvalidMigration) { t.Fatalf("error = %v, want ErrInvalidMigration", err) }
}

func TestDiscoverRejectsInvalidNameAndEmptySQL(t *testing.T) {
	cases := []struct{name string; data string}{
		{"001_bad-name.up.sql", "SELECT 1;"},
		{"001_empty.up.sql", "  \n  "},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			files := fstest.MapFS{"migrations/"+tc.name: {Data: []byte(tc.data)}}
			_, err := Discover(files, "migrations")
			if !errors.Is(err, ErrInvalidMigration) { t.Fatalf("error = %v, want ErrInvalidMigration", err) }
		})
	}
}

func TestSchemaNameValidation(t *testing.T) {
	valid := []string{"public", "app_data", "tenant_123"}
	for _, name := range valid {
		if !schemaPattern.MatchString(name) { t.Errorf("%q should be valid", name) }
	}
	invalid := []string{"", "Public", "x; DROP SCHEMA public", "a.b", "1tenant", "a-b"}
	for _, name := range invalid {
		if schemaPattern.MatchString(name) { t.Errorf("%q should be invalid", name) }
	}
}

type fakeMigrationState struct {
	mu         sync.Mutex
	ledger     map[int64]AppliedMigration
	ledgerSet  bool
	statements []string
}
type fakeMigrationDriver struct{ state *fakeMigrationState }
type fakeMigrationConn struct { state *fakeMigrationState; active *fakeMigrationTx }
type fakeMigrationTx struct {
	state     *fakeMigrationState
	conn      *fakeMigrationConn
	pending   *AppliedMigration
	statement string
	finished  bool
}
type fakeMigrationRows struct {
	columns []string
	values  [][]driver.Value
	index   int
}

func (d fakeMigrationDriver) Open(string) (driver.Conn, error) {
	return &fakeMigrationConn{state: d.state}, nil
}
func (c *fakeMigrationConn) Prepare(string) (driver.Stmt, error) {
	return nil, errors.New("prepared statements are not supported by fake driver")
}
func (c *fakeMigrationConn) Close() error { return nil }
func (c *fakeMigrationConn) Begin() (driver.Tx, error) {
	return c.BeginTx(context.Background(), driver.TxOptions{})
}
func (c *fakeMigrationConn) BeginTx(context.Context, driver.TxOptions) (driver.Tx, error) {
	tx := &fakeMigrationTx{state: c.state, conn: c}
	c.active = tx
	return tx, nil
}
func (c *fakeMigrationConn) ExecContext(ctx context.Context, query string, args []driver.NamedValue) (driver.Result, error) {
	if c.active != nil { return c.active.ExecContext(ctx, query, args) }
	upper := strings.ToUpper(strings.TrimSpace(query))
	switch {
	case strings.HasPrefix(upper, "CREATE SCHEMA"), strings.HasPrefix(upper, "SET SEARCH_PATH"),
		strings.Contains(upper, "PG_ADVISORY_LOCK"), strings.Contains(upper, "PG_ADVISORY_UNLOCK"):
		return driver.RowsAffected(1), nil
	case strings.HasPrefix(upper, "CREATE TABLE IF NOT EXISTS SCHEMA_MIGRATIONS"):
		c.state.mu.Lock()
		c.state.ledgerSet = true
		c.state.mu.Unlock()
		return driver.RowsAffected(0), nil
	default:
		return nil, fmt.Errorf("unexpected fake ExecContext query: %s", query)
	}
}
func (c *fakeMigrationConn) QueryContext(_ context.Context, query string, _ []driver.NamedValue) (driver.Rows, error) {
	upper := strings.ToUpper(strings.TrimSpace(query))
	if strings.Contains(upper, "TO_REGCLASS") {
		c.state.mu.Lock()
		exists := c.state.ledgerSet
		c.state.mu.Unlock()
		return &fakeMigrationRows{columns: []string{"exists"}, values: [][]driver.Value{{exists}}}, nil
	}
	if strings.Contains(upper, "FROM SCHEMA_MIGRATIONS") {
		c.state.mu.Lock()
		defer c.state.mu.Unlock()
		versions := make([]int64, 0, len(c.state.ledger))
		for version := range c.state.ledger { versions = append(versions, version) }
		sort.Slice(versions, func(i, j int) bool { return versions[i] < versions[j] })
		values := make([][]driver.Value, 0, len(versions))
		for _, version := range versions {
			item := c.state.ledger[version]
			values = append(values, []driver.Value{item.Version, item.Name, item.Checksum, item.AppliedAt})
		}
		return &fakeMigrationRows{columns: []string{"version", "name", "checksum", "applied_at"}, values: values}, nil
	}
	return nil, fmt.Errorf("unexpected fake QueryContext query: %s", query)
}
func (tx *fakeMigrationTx) ExecContext(_ context.Context, query string, args []driver.NamedValue) (driver.Result, error) {
	trimmed := strings.TrimSpace(query)
	upper := strings.ToUpper(trimmed)
	if strings.HasPrefix(upper, "INSERT INTO SCHEMA_MIGRATIONS") {
		if len(args) != 3 { return nil, fmt.Errorf("expected 3 ledger args, got %d", len(args)) }
		version, ok := args[0].Value.(int64)
		if !ok { return nil, fmt.Errorf("version argument has type %T", args[0].Value) }
		name, _ := args[1].Value.(string)
		checksum, _ := args[2].Value.(string)
		tx.pending = &AppliedMigration{Version: version, Name: name, Checksum: checksum, AppliedAt: time.Now().UTC()}
		return driver.RowsAffected(1), nil
	}
	if strings.HasPrefix(upper, "BROKEN") { return nil, errors.New("simulated SQL syntax error") }
	tx.statement = trimmed
	return driver.RowsAffected(0), nil
}
func (tx *fakeMigrationTx) Commit() error {
	if tx.finished { return errors.New("transaction already finished") }
	tx.finished = true
	tx.conn.active = nil
	tx.state.mu.Lock()
	defer tx.state.mu.Unlock()
	if tx.pending != nil { tx.state.ledger[tx.pending.Version] = *tx.pending }
	if tx.statement != "" { tx.state.statements = append(tx.state.statements, tx.statement) }
	return nil
}
func (tx *fakeMigrationTx) Rollback() error {
	if tx.finished { return nil }
	tx.finished = true
	tx.conn.active = nil
	return nil
}
func (r *fakeMigrationRows) Columns() []string { return r.columns }
func (r *fakeMigrationRows) Close() error { return nil }
func (r *fakeMigrationRows) Next(dest []driver.Value) error {
	if r.index >= len(r.values) { return io.EOF }
	copy(dest, r.values[r.index])
	r.index++
	return nil
}

func TestRunnerUpIsIdempotentAndDetectsDriftWithFakeDatabase(t *testing.T) {
	state := &fakeMigrationState{ledger: make(map[int64]AppliedMigration)}
	const driverName = "day20-migration-fake"
	sql.Register(driverName, fakeMigrationDriver{state: state})
	db, err := sql.Open(driverName, "")
	if err != nil { t.Fatal(err) }
	defer db.Close()
	files := fstest.MapFS{
		"migrations/001_create_probe.up.sql": {Data: []byte("CREATE TABLE probe (id BIGINT PRIMARY KEY);")},
		"migrations/002_add_name.up.sql": {Data: []byte("ALTER TABLE probe ADD COLUMN name TEXT;")},
	}
	runner := Runner{DB: db, FS: files, Dir: "migrations", Schema: "test_schema", LockID: 100}
	ctx := context.Background()
	if err := runner.Up(ctx); err != nil { t.Fatal(err) }
	if err := runner.Up(ctx); err != nil { t.Fatalf("second Up: %v", err) }
	status, err := runner.Status(ctx)
	if err != nil { t.Fatal(err) }
	if len(status) != 2 { t.Fatalf("status count = %d, want 2", len(status)) }

	changed := fstest.MapFS{
		"migrations/001_create_probe.up.sql": {Data: []byte("CREATE TABLE probe (id BIGINT PRIMARY KEY, changed BOOLEAN);")},
		"migrations/002_add_name.up.sql": {Data: []byte("ALTER TABLE probe ADD COLUMN name TEXT;")},
	}
	runner.FS = changed
	if err := runner.Up(ctx); !errors.Is(err, ErrChecksumMismatch) { t.Fatalf("drift error = %v, want ErrChecksumMismatch", err) }
}

func TestRunnerDoesNotRecordFailedMigration(t *testing.T) {
	state := &fakeMigrationState{ledger: make(map[int64]AppliedMigration)}
	const driverName = "day20-migration-fake-failure"
	sql.Register(driverName, fakeMigrationDriver{state: state})
	db, err := sql.Open(driverName, "")
	if err != nil { t.Fatal(err) }
	defer db.Close()
	files := fstest.MapFS{
		"migrations/001_broken.up.sql": {Data: []byte("BROKEN SQL;")},
	}
	runner := Runner{DB: db, FS: files, Dir: "migrations", Schema: "test_schema", LockID: 101}
	if err := runner.Up(context.Background()); err == nil { t.Fatal("expected migration execution failure") }
	state.mu.Lock()
	defer state.mu.Unlock()
	if len(state.ledger) != 0 { t.Fatalf("failed migration was recorded: %+v", state.ledger) }
}
