package main

import (
	"context"
	"errors"
	"fmt"
	"strings"
)

var (
	ErrInvalidProduct = errors.New("invalid product")
	ErrNotFound       = errors.New("product not found")
)

type Product struct {
	ID         int64  `json:"id"`
	SKU        string `json:"sku"`
	Name       string `json:"name"`
	PricePaise int64  `json:"price_paise"`
	Stock      int64  `json:"stock"`
}

type CreateProductInput struct {
	SKU        string `json:"sku"`
	Name       string `json:"name"`
	PricePaise int64  `json:"price_paise"`
	Stock      int64  `json:"stock"`
}

type ProductRepository interface {
	Create(ctx context.Context, input CreateProductInput) (Product, error)
	Get(ctx context.Context, id int64) (Product, error)
	List(ctx context.Context, limit, offset int) ([]Product, error)
}

type ProductService struct {
	repo ProductRepository
}

func NewProductService(repo ProductRepository) *ProductService {
	return &ProductService{repo: repo}
}

func (s *ProductService) Create(ctx context.Context, input CreateProductInput) (Product, error) {
	input.SKU = strings.TrimSpace(input.SKU)
	input.Name = strings.TrimSpace(input.Name)
	if input.SKU == "" || input.Name == "" || input.PricePaise <= 0 || input.Stock < 0 {
		return Product{}, ErrInvalidProduct
	}
	product, err := s.repo.Create(ctx, input)
	if err != nil {
		return Product{}, fmt.Errorf("create product: %w", err)
	}
	return product, nil
}

func (s *ProductService) Get(ctx context.Context, id int64) (Product, error) {
	if id <= 0 {
		return Product{}, ErrInvalidProduct
	}
	product, err := s.repo.Get(ctx, id)
	if err != nil {
		return Product{}, fmt.Errorf("get product %d: %w", id, err)
	}
	return product, nil
}

func (s *ProductService) List(ctx context.Context, limit, offset int) ([]Product, error) {
	if limit < 1 || limit > 100 || offset < 0 {
		return nil, ErrInvalidProduct
	}
	products, err := s.repo.List(ctx, limit, offset)
	if err != nil {
		return nil, fmt.Errorf("list products: %w", err)
	}
	return products, nil
}
