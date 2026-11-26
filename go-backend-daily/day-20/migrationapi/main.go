package main

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"strconv"
	"time"
)

func main() {
	if err := run(); err != nil {
		fmt.Fprintln(os.Stderr, "migration command failed:", err)
		os.Exit(1)
	}
}

func run() error {
	if len(os.Args) != 2 || (os.Args[1] != "up" && os.Args[1] != "status") {
		return errors.New("usage: go run -tags pgx ./day-20/migrationapi <up|status>")
	}
	maxOpen, err := envInt("DB_MAX_OPEN_CONNS", 5)
	if err != nil {
		return err
	}
	maxIdle, err := envInt("DB_MAX_IDLE_CONNS", 2)
	if err != nil {
		return err
	}
	cfg := DBConfig{
		URL:             os.Getenv("DATABASE_URL"),
		MaxOpenConns:    maxOpen,
		MaxIdleConns:    maxIdle,
		ConnMaxLifetime: 30 * time.Minute,
		PingTimeout:     5 * time.Second,
	}
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
	defer cancel()
	db, err := OpenDB(ctx, cfg)
	if err != nil {
		return err
	}
	defer db.Close()

	runner := Runner{
		DB:     db,
		FS:     embeddedMigrations,
		Dir:    "migrations",
		Schema: envOr("MIGRATION_SCHEMA", "public"),
		LockID: 2026112601,
	}
	if os.Args[1] == "up" {
		if err := runner.Up(ctx); err != nil {
			return err
		}
		fmt.Println("database migrations are up to date")
		return nil
	}
	items, err := runner.Status(ctx)
	if err != nil {
		return err
	}
	encoder := json.NewEncoder(os.Stdout)
	encoder.SetIndent("", "  ")
	return encoder.Encode(items)
}

func envOr(key, fallback string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return fallback
}

func envInt(key string, fallback int) (int, error) {
	value := os.Getenv(key)
	if value == "" {
		return fallback, nil
	}
	parsed, err := strconv.Atoi(value)
	if err != nil {
		return 0, fmt.Errorf("%s must be an integer: %w", key, err)
	}
	return parsed, nil
}
