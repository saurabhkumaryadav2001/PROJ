//go:build integration && pgx

package main

import (
	"context"
	"database/sql"
	"errors"
	"os"
	"testing"
	"time"

	_ "github.com/jackc/pgx/v5/stdlib"
)

// Uses a PostgreSQL temporary table on one dedicated connection.
func TestPostgresRepositoryIntegration(t *testing.T) {
	dsn := os.Getenv("TEST_DATABASE_URL")
	if dsn == "" {
		t.Skip("set TEST_DATABASE_URL to run PostgreSQL integration test")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	db, err := sql.Open("pgx", dsn)
	if err != nil { t.Fatal(err) }
	defer db.Close()
	conn, err := db.Conn(ctx)
	if err != nil { t.Fatal(err) }
	defer conn.Close()

	_, err = conn.ExecContext(ctx, `
		CREATE TEMP TABLE products (
			id BIGSERIAL PRIMARY KEY,
			sku TEXT NOT NULL UNIQUE,
			name TEXT NOT NULL,
			price_paise BIGINT NOT NULL,
			stock BIGINT NOT NULL
		)`)
	if err != nil { t.Fatal(err) }

	repository := NewSQLRepository(conn)
	input := CreateProductInput{SKU: "IT-1", Name: "Integration product", PricePaise: 12345, Stock: 7}
	created, err := repository.Create(ctx, input)
	if err != nil { t.Fatal(err) }
	if created.ID != 1 || created.SKU != input.SKU || created.PricePaise != input.PricePaise {
		t.Fatalf("unexpected created product: %+v", created)
	}
	got, err := repository.Get(ctx, created.ID)
	if err != nil || got != created { t.Fatalf("Get() = %+v, %v; want %+v, nil", got, err, created) }
	if _, err := repository.Get(ctx, 999); !errors.Is(err, ErrNotFound) {
		t.Fatalf("missing product error = %v, want ErrNotFound", err)
	}
	items, err := repository.List(ctx, 10, 0)
	if err != nil || len(items) != 1 || items[0] != created {
		t.Fatalf("List() = %+v, %v", items, err)
	}
}
