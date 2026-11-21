package main

import (
	"context"
	"errors"
	"testing"
)

type fakeRepository struct {
	createFn func(context.Context, CreateProductInput) (Product, error)
	getFn    func(context.Context, int64) (Product, error)
	listFn   func(context.Context, int, int) ([]Product, error)
	creates  int
}

func (f *fakeRepository) Create(ctx context.Context, input CreateProductInput) (Product, error) {
	f.creates++
	if f.createFn != nil {
		return f.createFn(ctx, input)
	}
	return Product{ID: 1, SKU: input.SKU, Name: input.Name, PricePaise: input.PricePaise, Stock: input.Stock}, nil
}
func (f *fakeRepository) Get(ctx context.Context, id int64) (Product, error) {
	if f.getFn != nil { return f.getFn(ctx, id) }
	return Product{}, ErrNotFound
}
func (f *fakeRepository) List(ctx context.Context, limit, offset int) ([]Product, error) {
	if f.listFn != nil { return f.listFn(ctx, limit, offset) }
	return []Product{}, nil
}

func TestProductServiceCreateValidation(t *testing.T) {
	cases := []struct {
		name string
		input CreateProductInput
	}{
		{name: "empty SKU", input: CreateProductInput{Name: "Keyboard", PricePaise: 100, Stock: 1}},
		{name: "empty name", input: CreateProductInput{SKU: "KB-1", PricePaise: 100, Stock: 1}},
		{name: "zero price", input: CreateProductInput{SKU: "KB-1", Name: "Keyboard", PricePaise: 0, Stock: 1}},
		{name: "negative price", input: CreateProductInput{SKU: "KB-1", Name: "Keyboard", PricePaise: -1, Stock: 1}},
		{name: "negative stock", input: CreateProductInput{SKU: "KB-1", Name: "Keyboard", PricePaise: 100, Stock: -1}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			repo := &fakeRepository{}
			service := NewProductService(repo)
			_, err := service.Create(context.Background(), tc.input)
			if !errors.Is(err, ErrInvalidProduct) { t.Fatalf("error = %v, want ErrInvalidProduct", err) }
			if repo.creates != 0 { t.Fatalf("repository called %d times for invalid input", repo.creates) }
		})
	}
}

func TestProductServiceCreateTrimsFields(t *testing.T) {
	service := NewProductService(&fakeRepository{})
	got, err := service.Create(context.Background(), CreateProductInput{
		SKU: "  KB-1  ", Name: "  Keyboard  ", PricePaise: 149900, Stock: 5,
	})
	if err != nil { t.Fatal(err) }
	if got.SKU != "KB-1" || got.Name != "Keyboard" { t.Fatalf("fields were not trimmed: %+v", got) }
}

func TestProductServiceWrapsRepositoryError(t *testing.T) {
	databaseErr := errors.New("database unavailable")
	repo := &fakeRepository{createFn: func(context.Context, CreateProductInput) (Product, error) {
		return Product{}, databaseErr
	}}
	service := NewProductService(repo)
	_, err := service.Create(context.Background(), CreateProductInput{
		SKU: "KB-1", Name: "Keyboard", PricePaise: 100, Stock: 1,
	})
	if !errors.Is(err, databaseErr) { t.Fatalf("error = %v, want wrapped database error", err) }
}

func TestProductServiceGetAndList(t *testing.T) {
	want := Product{ID: 7, SKU: "M-7", Name: "Mouse", PricePaise: 79900, Stock: 3}
	repo := &fakeRepository{
		getFn: func(_ context.Context, id int64) (Product, error) {
			if id != want.ID { t.Fatalf("repository received id %d, want %d", id, want.ID) }
			return want, nil
		},
		listFn: func(_ context.Context, limit, offset int) ([]Product, error) {
			if limit != 10 || offset != 20 { t.Fatalf("repository received limit=%d offset=%d", limit, offset) }
			return []Product{want}, nil
		},
	}
	service := NewProductService(repo)
	got, err := service.Get(context.Background(), want.ID)
	if err != nil || got != want { t.Fatalf("Get() = %+v, %v; want %+v, nil", got, err, want) }
	items, err := service.List(context.Background(), 10, 20)
	if err != nil || len(items) != 1 || items[0] != want { t.Fatalf("List() = %+v, %v", items, err) }
}

func TestProductServiceRejectsInvalidPagination(t *testing.T) {
	cases := []struct { name string; limit, offset int }{
		{name: "zero limit", limit: 0, offset: 0},
		{name: "limit too large", limit: 101, offset: 0},
		{name: "negative offset", limit: 10, offset: -1},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			service := NewProductService(&fakeRepository{})
			_, err := service.List(context.Background(), tc.limit, tc.offset)
			if !errors.Is(err, ErrInvalidProduct) { t.Fatalf("error = %v, want ErrInvalidProduct", err) }
		})
	}
}
