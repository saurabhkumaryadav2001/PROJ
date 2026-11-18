package main

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"math"
	"time"
)

var (
	ErrNotFound          = errors.New("resource not found")
	ErrInsufficientStock = errors.New("insufficient stock")
)

type Product struct {
	ID         int64     `json:"id"`
	SKU        string    `json:"sku"`
	Name       string    `json:"name"`
	PricePaise int64     `json:"price_paise"`
	Stock      int       `json:"stock"`
	CreatedAt  time.Time `json:"created_at"`
}

type CreateProductInput struct {
	SKU        string `json:"sku"`
	Name       string `json:"name"`
	PricePaise int64  `json:"price_paise"`
	Stock      int    `json:"stock"`
}

type Order struct {
	ID             int64     `json:"id"`
	ProductID      int64     `json:"product_id"`
	Quantity       int       `json:"quantity"`
	UnitPricePaise int64     `json:"unit_price_paise"`
	TotalPaise     int64     `json:"total_paise"`
	CreatedAt      time.Time `json:"created_at"`
}

type CreateOrderInput struct {
	ProductID int64 `json:"product_id"`
	Quantity  int   `json:"quantity"`
}

type Repository interface {
	Ping(context.Context) error
	CreateProduct(context.Context, CreateProductInput) (Product, error)
	GetProduct(context.Context, int64) (Product, error)
	ListProducts(context.Context, int, int) ([]Product, error)
	CreateOrder(context.Context, CreateOrderInput) (Order, error)
}

type SQLRepository struct {
	db *sql.DB
}

func NewSQLRepository(db *sql.DB) *SQLRepository {
	return &SQLRepository{db: db}
}

func (r *SQLRepository) Ping(ctx context.Context) error {
	return r.db.PingContext(ctx)
}

func (r *SQLRepository) CreateProduct(ctx context.Context, input CreateProductInput) (Product, error) {
	const query = `
		INSERT INTO products (sku, name, price_paise, stock)
		VALUES ($1, $2, $3, $4)
		RETURNING id, sku, name, price_paise, stock, created_at`

	var product Product
	err := r.db.QueryRowContext(ctx, query,
		input.SKU, input.Name, input.PricePaise, input.Stock,
	).Scan(
		&product.ID, &product.SKU, &product.Name,
		&product.PricePaise, &product.Stock, &product.CreatedAt,
	)
	if err != nil {
		return Product{}, fmt.Errorf("insert product: %w", err)
	}
	return product, nil
}

func (r *SQLRepository) GetProduct(ctx context.Context, id int64) (Product, error) {
	const query = `
		SELECT id, sku, name, price_paise, stock, created_at
		FROM products WHERE id = $1`

	var product Product
	err := r.db.QueryRowContext(ctx, query, id).Scan(
		&product.ID, &product.SKU, &product.Name,
		&product.PricePaise, &product.Stock, &product.CreatedAt,
	)
	if errors.Is(err, sql.ErrNoRows) {
		return Product{}, ErrNotFound
	}
	if err != nil {
		return Product{}, fmt.Errorf("get product: %w", err)
	}
	return product, nil
}

func (r *SQLRepository) ListProducts(ctx context.Context, limit, offset int) ([]Product, error) {
	const query = `
		SELECT id, sku, name, price_paise, stock, created_at
		FROM products
		ORDER BY id
		LIMIT $1 OFFSET $2`

	rows, err := r.db.QueryContext(ctx, query, limit, offset)
	if err != nil {
		return nil, fmt.Errorf("list products: %w", err)
	}
	defer rows.Close()

	products := make([]Product, 0)
	for rows.Next() {
		var product Product
		if err := rows.Scan(
			&product.ID, &product.SKU, &product.Name,
			&product.PricePaise, &product.Stock, &product.CreatedAt,
		); err != nil {
			return nil, fmt.Errorf("scan product: %w", err)
		}
		products = append(products, product)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate products: %w", err)
	}
	return products, nil
}

// CreateOrder uses one transaction for stock decrement + order insertion.
// The conditional UPDATE prevents two concurrent requests from overselling stock.
func (r *SQLRepository) CreateOrder(ctx context.Context, input CreateOrderInput) (Order, error) {
	tx, err := r.db.BeginTx(ctx, &sql.TxOptions{Isolation: sql.LevelReadCommitted})
	if err != nil {
		return Order{}, fmt.Errorf("begin order transaction: %w", err)
	}
	defer func() { _ = tx.Rollback() }() // harmless after a successful Commit

	const reserveStock = `
		UPDATE products
		SET stock = stock - $1
		WHERE id = $2 AND stock >= $1
		RETURNING price_paise`

	var unitPrice int64
	err = tx.QueryRowContext(ctx, reserveStock, input.Quantity, input.ProductID).Scan(&unitPrice)
	if errors.Is(err, sql.ErrNoRows) {
		var exists bool
		if checkErr := tx.QueryRowContext(ctx,
			`SELECT EXISTS (SELECT 1 FROM products WHERE id = $1)`, input.ProductID,
		).Scan(&exists); checkErr != nil {
			return Order{}, fmt.Errorf("check product after stock reservation: %w", checkErr)
		}
		if !exists {
			return Order{}, ErrNotFound
		}
		return Order{}, ErrInsufficientStock
	}
	if err != nil {
		return Order{}, fmt.Errorf("reserve product stock: %w", err)
	}

	if int64(input.Quantity) > math.MaxInt64/unitPrice {
		return Order{}, errors.New("order total exceeds supported amount")
	}
	total := unitPrice * int64(input.Quantity)

	const insertOrder = `
		INSERT INTO orders (product_id, quantity, unit_price_paise, total_paise)
		VALUES ($1, $2, $3, $4)
		RETURNING id, created_at`

	order := Order{
		ProductID:      input.ProductID,
		Quantity:       input.Quantity,
		UnitPricePaise: unitPrice,
		TotalPaise:     total,
	}
	if err := tx.QueryRowContext(ctx, insertOrder,
		order.ProductID, order.Quantity, order.UnitPricePaise, order.TotalPaise,
	).Scan(&order.ID, &order.CreatedAt); err != nil {
		return Order{}, fmt.Errorf("insert order: %w", err)
	}

	if err := tx.Commit(); err != nil {
		return Order{}, fmt.Errorf("commit order transaction: %w", err)
	}
	return order, nil
}
