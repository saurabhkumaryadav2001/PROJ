package main

import (
	"context"
	"crypto/sha256"
	"database/sql"
	"embed"
	"encoding/hex"
	"errors"
	"fmt"
	"io/fs"
	"path"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"time"
)

//go:embed migrations/*.up.sql
var embeddedMigrations embed.FS

var (
	ErrChecksumMismatch = errors.New("applied migration checksum does not match source")
	ErrInvalidMigration = errors.New("invalid migration file")
)

var migrationNamePattern = regexp.MustCompile(`^(\d{3,})_([a-z0-9_]+)\.up\.sql$`)
var schemaPattern = regexp.MustCompile(`^[a-z][a-z0-9_]{0,62}$`)

type Migration struct {
	Version  int64
	Name     string
	SQL      string
	Checksum string
}

type AppliedMigration struct {
	Version   int64     `json:"version"`
	Name      string    `json:"name"`
	Checksum  string    `json:"checksum"`
	AppliedAt time.Time `json:"applied_at"`
}

type Runner struct {
	DB     *sql.DB
	FS     fs.FS
	Dir    string
	Schema string
	LockID int64
}

// Discover reads, validates and sorts migration files. Each file must contain
// exactly one database statement; split complex changes into ordered versions.
func Discover(source fs.FS, dir string) ([]Migration, error) {
	if source == nil {
		return nil, fmt.Errorf("migration filesystem is nil")
	}
	matches, err := fs.Glob(source, path.Join(dir, "*.up.sql"))
	if err != nil {
		return nil, fmt.Errorf("list migration files: %w", err)
	}
	migrations := make([]Migration, 0, len(matches))
	seen := make(map[int64]struct{}, len(matches))
	for _, file := range matches {
		base := path.Base(file)
		parts := migrationNamePattern.FindStringSubmatch(base)
		if parts == nil {
			return nil, fmt.Errorf("%w: %s (expected NNN_name.up.sql)", ErrInvalidMigration, base)
		}
		version, err := strconv.ParseInt(parts[1], 10, 64)
		if err != nil || version <= 0 {
			return nil, fmt.Errorf("%w: invalid version in %s", ErrInvalidMigration, base)
		}
		if _, exists := seen[version]; exists {
			return nil, fmt.Errorf("%w: duplicate version %d", ErrInvalidMigration, version)
		}
		seen[version] = struct{}{}
		contents, err := fs.ReadFile(source, file)
		if err != nil {
			return nil, fmt.Errorf("read %s: %w", file, err)
		}
		sqlText := strings.TrimSpace(string(contents))
		if sqlText == "" {
			return nil, fmt.Errorf("%w: %s is empty", ErrInvalidMigration, base)
		}
		sum := sha256.Sum256([]byte(sqlText))
		migrations = append(migrations, Migration{
			Version:  version,
			Name:     strings.TrimSuffix(base, ".up.sql"),
			SQL:      sqlText,
			Checksum: hex.EncodeToString(sum[:]),
		})
	}
	sort.Slice(migrations, func(i, j int) bool {
		return migrations[i].Version < migrations[j].Version
	})
	return migrations, nil
}

func (r Runner) validate() error {
	if r.DB == nil {
		return errors.New("database is required")
	}
	if r.FS == nil {
		return errors.New("migration filesystem is required")
	}
	if r.Dir == "" {
		return errors.New("migration directory is required")
	}
	if r.Schema != "" && !schemaPattern.MatchString(r.Schema) {
		return fmt.Errorf("invalid PostgreSQL schema name %q", r.Schema)
	}
	return nil
}

func (r Runner) schemaName() string {
	if r.Schema == "" {
		return "public"
	}
	return r.Schema
}

func (r Runner) openScopedConn(ctx context.Context, createSchema bool) (*sql.Conn, error) {
	schema := r.schemaName()
	// Schema is restricted to a conservative identifier pattern before quoting.
	quoted := `"` + schema + `"`
	if createSchema {
		if _, err := r.DB.ExecContext(ctx, "CREATE SCHEMA IF NOT EXISTS "+quoted); err != nil {
			return nil, fmt.Errorf("create schema: %w", err)
		}
	}
	conn, err := r.DB.Conn(ctx)
	if err != nil {
		return nil, fmt.Errorf("get dedicated migration connection: %w", err)
	}
	if _, err := conn.ExecContext(ctx, "SET search_path TO "+quoted); err != nil {
		_ = conn.Close()
		return nil, fmt.Errorf("set migration search_path: %w", err)
	}
	return conn, nil
}

func (r Runner) Up(ctx context.Context) (returnErr error) {
	if err := r.validate(); err != nil {
		return err
	}
	migrations, err := Discover(r.FS, r.Dir)
	if err != nil {
		return err
	}
	conn, err := r.openScopedConn(ctx, true)
	if err != nil {
		return err
	}
	defer conn.Close()

	lockID := r.LockID
	if lockID == 0 {
		lockID = 2026112601
	}
	if _, err := conn.ExecContext(ctx, `SELECT pg_advisory_lock($1)`, lockID); err != nil {
		return fmt.Errorf("acquire migration advisory lock: %w", err)
	}
	defer func() {
		unlockCtx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
		defer cancel()
		if _, err := conn.ExecContext(unlockCtx, `SELECT pg_advisory_unlock($1)`, lockID); err != nil && returnErr == nil {
			returnErr = fmt.Errorf("release migration advisory lock: %w", err)
		}
	}()

	if _, err := conn.ExecContext(ctx, `
		CREATE TABLE IF NOT EXISTS schema_migrations (
			version BIGINT PRIMARY KEY,
			name TEXT NOT NULL,
			checksum CHAR(64) NOT NULL,
			applied_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
		)`); err != nil {
		return fmt.Errorf("create migration ledger: %w", err)
	}

	applied, err := readApplied(ctx, conn)
	if err != nil {
		return err
	}
	available := make(map[int64]Migration, len(migrations))
	for _, migration := range migrations {
		available[migration.Version] = migration
	}
	for version, existing := range applied {
		current, ok := available[version]
		if !ok {
			return fmt.Errorf("%w: applied version %d is missing from source", ErrChecksumMismatch, version)
		}
		if existing.Checksum != current.Checksum {
			return fmt.Errorf("%w: version %d (%s)", ErrChecksumMismatch, version, current.Name)
		}
	}
	for _, migration := range migrations {
		if _, ok := applied[migration.Version]; ok {
			continue
		}
		if err := applyOne(ctx, conn, migration); err != nil {
			return err
		}
	}
	return nil
}

func applyOne(ctx context.Context, conn *sql.Conn, migration Migration) error {
	tx, err := conn.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("begin migration %d: %w", migration.Version, err)
	}
	defer func() { _ = tx.Rollback() }()
	if _, err := tx.ExecContext(ctx, migration.SQL); err != nil {
		return fmt.Errorf("execute migration %d (%s): %w", migration.Version, migration.Name, err)
	}
	if _, err := tx.ExecContext(ctx,
		`INSERT INTO schema_migrations (version, name, checksum) VALUES ($1, $2, $3)`,
		migration.Version, migration.Name, migration.Checksum,
	); err != nil {
		return fmt.Errorf("record migration %d: %w", migration.Version, err)
	}
	if err := tx.Commit(); err != nil {
		return fmt.Errorf("commit migration %d: %w", migration.Version, err)
	}
	return nil
}

func readApplied(ctx context.Context, conn *sql.Conn) (map[int64]AppliedMigration, error) {
	rows, err := conn.QueryContext(ctx,
		`SELECT version, name, checksum, applied_at FROM schema_migrations ORDER BY version`)
	if err != nil {
		return nil, fmt.Errorf("read migration ledger: %w", err)
	}
	defer rows.Close()
	applied := make(map[int64]AppliedMigration)
	for rows.Next() {
		var item AppliedMigration
		if err := rows.Scan(&item.Version, &item.Name, &item.Checksum, &item.AppliedAt); err != nil {
			return nil, fmt.Errorf("scan migration ledger: %w", err)
		}
		applied[item.Version] = item
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate migration ledger: %w", err)
	}
	return applied, nil
}

func (r Runner) Status(ctx context.Context) ([]AppliedMigration, error) {
	if err := r.validate(); err != nil {
		return nil, err
	}
	conn, err := r.openScopedConn(ctx, false)
	if err != nil {
		return nil, err
	}
	defer conn.Close()
	var exists bool
	if err := conn.QueryRowContext(ctx,
		`SELECT to_regclass('schema_migrations') IS NOT NULL`).Scan(&exists); err != nil {
		return nil, fmt.Errorf("check migration ledger: %w", err)
	}
	if !exists {
		return []AppliedMigration{}, nil
	}
	applied, err := readApplied(ctx, conn)
	if err != nil {
		return nil, err
	}
	items := make([]AppliedMigration, 0, len(applied))
	for _, migration := range applied {
		items = append(items, migration)
	}
	sort.Slice(items, func(i, j int) bool { return items[i].Version < items[j].Version })
	return items, nil
}
