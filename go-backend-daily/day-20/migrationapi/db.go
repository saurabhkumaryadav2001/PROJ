package main

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"time"
)

type DBConfig struct {
	URL             string
	MaxOpenConns    int
	MaxIdleConns    int
	ConnMaxLifetime time.Duration
	PingTimeout     time.Duration
}

func OpenDB(ctx context.Context, cfg DBConfig) (*sql.DB, error) {
	if cfg.URL == "" {
		return nil, errors.New("DATABASE_URL is required")
	}
	if cfg.MaxOpenConns <= 0 || cfg.MaxIdleConns < 0 || cfg.MaxIdleConns > cfg.MaxOpenConns {
		return nil, errors.New("invalid database pool limits")
	}
	if cfg.ConnMaxLifetime <= 0 || cfg.PingTimeout <= 0 {
		return nil, errors.New("database lifetime and ping timeout must be positive")
	}
	db, err := sql.Open("pgx", cfg.URL)
	if err != nil {
		return nil, fmt.Errorf("open PostgreSQL pool: %w", err)
	}
	db.SetMaxOpenConns(cfg.MaxOpenConns)
	db.SetMaxIdleConns(cfg.MaxIdleConns)
	db.SetConnMaxLifetime(cfg.ConnMaxLifetime)
	db.SetConnMaxIdleTime(5 * time.Minute)

	pingCtx, cancel := context.WithTimeout(ctx, cfg.PingTimeout)
	defer cancel()
	if err := db.PingContext(pingCtx); err != nil {
		_ = db.Close()
		return nil, fmt.Errorf("ping PostgreSQL: %w", err)
	}
	return db, nil
}
