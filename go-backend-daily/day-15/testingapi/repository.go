package main

import (
	"context"
	"database/sql"
	"sort"
	"sync"
)

// MemoryRepository is a runnable local adapter. Production deployments can use SQLRepository.
type MemoryRepository struct {
	mu       sync.RWMutex
	products map[int64]Product
	nextID   int64
}

func NewMemoryRepository() *MemoryRepository {
	return &MemoryRepository{products: make(map[int64]Product), nextID: 1}
}

func (r *MemoryRepository) Create(ctx context.Context, input CreateProductInput) (Product, error) {
	if err := ctx.Err(); err != nil {
		return Product{}, err
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	product := Product{ID: r.nextID, SKU: input.SKU, Name: input.Name, PricePaise: input.PricePaise, Stock: input.Stock}
	r.products[product.ID] = product
	r.nextID++
	return product, nil
}

func (r *MemoryRepository) Get(ctx context.Context, id int64) (Product, error) {
	if err := ctx.Err(); err != nil {
		return Product{}, err
	}
	r.mu.RLock()
	defer r.mu.RUnlock()
	product, ok := r.products[id]
	if !ok {
		return Product{}, ErrNotFound
	}
	return product, nil
}

func (r *MemoryRepository) List(ctx context.Context, limit, offset int) ([]Product, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	r.mu.RLock()
	defer r.mu.RUnlock()
	ids := make([]int64, 0, len(r.products))
	for id := range r.products {
		ids = append(ids, id)
	}
	sort.Slice(ids, func(i, j int) bool { return ids[i] < ids[j] })
	if offset >= len(ids) {
		return []Product{}, nil
	}
	ids = ids[offset:]
	if len(ids) > limit {
		ids = ids[:limit]
	}
	products := make([]Product, 0, len(ids))
	for _, id := range ids {
		products = append(products, r.products[id])
	}
	return products, nil
}

// DBTX is implemented by *sql.DB, *sql.Tx and *sql.Conn.
type DBTX interface {
	ExecContext(context.Context, string, ...any) (sql.Result, error)
	QueryContext(context.Context, string, ...any) (*sql.Rows, error)
	QueryRowContext(context.Context, string, ...any) *sql.Row
}

type SQLRepository struct{ db DBTX }

func NewSQLRepository(db DBTX) *SQLRepository { return &SQLRepository{db: db} }

func (r *SQLRepository) Create(ctx context.Context, input CreateProductInput) (Product, error) {
	const query = `INSERT INTO products (sku, name, price_paise, stock)
		VALUES ($1, $2, $3, $4)
		RETURNING id, sku, name, price_paise, stock`
	var product Product
	err := r.db.QueryRowContext(ctx, query, input.SKU, input.Name, input.PricePaise, input.Stock).
		Scan(&product.ID, &product.SKU, &product.Name, &product.PricePaise, &product.Stock)
	return product, err
}

func (r *SQLRepository) Get(ctx context.Context, id int64) (Product, error) {
	const query = `SELECT id, sku, name, price_paise, stock FROM products WHERE id = $1`
	var product Product
	err := r.db.QueryRowContext(ctx, query, id).
		Scan(&product.ID, &product.SKU, &product.Name, &product.PricePaise, &product.Stock)
	if err == sql.ErrNoRows {
		return Product{}, ErrNotFound
	}
	return product, err
}

func (r *SQLRepository) List(ctx context.Context, limit, offset int) ([]Product, error) {
	const query = `SELECT id, sku, name, price_paise, stock FROM products ORDER BY id LIMIT $1 OFFSET $2`
	rows, err := r.db.QueryContext(ctx, query, limit, offset)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	products := make([]Product, 0)
	for rows.Next() {
		var product Product
		if err := rows.Scan(&product.ID, &product.SKU, &product.Name, &product.PricePaise, &product.Stock); err != nil {
			return nil, err
		}
		products = append(products, product)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	return products, nil
}

var _ ProductRepository = (*MemoryRepository)(nil)
var _ ProductRepository = (*SQLRepository)(nil)
