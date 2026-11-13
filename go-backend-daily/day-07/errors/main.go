package main

import (
	"errors"
	"fmt"
)

var (
	ErrProductNotFound    = errors.New("product not found")
	ErrInsufficientStock = errors.New("insufficient stock")
)

type ValidationError struct {
	Field   string
	Message string
}

func (e *ValidationError) Error() string {
	return fmt.Sprintf("%s: %s", e.Field, e.Message)
}

type Product struct {
	ID    int
	Name  string
	Stock int
}

type Order struct {
	ProductID int
	Quantity  int
}

type ProductRepository struct {
	products map[int]Product
}

func NewProductRepository() *ProductRepository {
	return &ProductRepository{products: map[int]Product{
		1: {ID: 1, Name: "Keyboard", Stock: 5},
		2: {ID: 2, Name: "Mouse", Stock: 0},
	}}
}

func (r *ProductRepository) GetByID(id int) (Product, error) {
	product, ok := r.products[id]
	if !ok {
		return Product{}, fmt.Errorf("get product %d: %w", id, ErrProductNotFound)
	}
	return product, nil
}

func (r *ProductRepository) ReduceStock(id, quantity int) error {
	product, err := r.GetByID(id)
	if err != nil {
		return err
	}
	if quantity > product.Stock {
		return fmt.Errorf("product %d has %d units, requested %d: %w", id, product.Stock, quantity, ErrInsufficientStock)
	}
	product.Stock -= quantity
	r.products[id] = product
	return nil
}

type OrderService struct {
	products *ProductRepository
}

func NewOrderService(products *ProductRepository) *OrderService {
	return &OrderService{products: products}
}

func (s *OrderService) CreateOrder(productID, quantity int) (Order, error) {
	if productID <= 0 {
		return Order{}, &ValidationError{Field: "productID", Message: "must be positive"}
	}
	if quantity <= 0 {
		return Order{}, &ValidationError{Field: "quantity", Message: "must be positive"}
	}

	if _, err := s.products.GetByID(productID); err != nil {
		return Order{}, fmt.Errorf("order validation: %w", err)
	}
	if err := s.products.ReduceStock(productID, quantity); err != nil {
		return Order{}, fmt.Errorf("reserve inventory: %w", err)
	}

	return Order{ProductID: productID, Quantity: quantity}, nil
}

// httpStatusForError demonstrates mapping domain errors to HTTP status codes.
// The example intentionally avoids importing net/http so it stays focused.
func httpStatusForError(err error) int {
	var validationErr *ValidationError
	switch {
	case errors.As(err, &validationErr):
		return 400 // Bad Request
	case errors.Is(err, ErrProductNotFound):
		return 404 // Not Found
	case errors.Is(err, ErrInsufficientStock):
		return 409 // Conflict
	default:
		return 500 // Internal Server Error
	}
}

func report(label string, err error) {
	if err == nil {
		fmt.Printf("%s: success\n", label)
		return
	}
	fmt.Printf("%s: %v (HTTP mapping %d)\n", label, err, httpStatusForError(err))

	if errors.Is(err, ErrProductNotFound) {
		fmt.Println("  errors.Is: product-not-found identified")
	}
	if errors.Is(err, ErrInsufficientStock) {
		fmt.Println("  errors.Is: insufficient-stock identified")
	}
	var validationErr *ValidationError
	if errors.As(err, &validationErr) {
		fmt.Printf("  errors.As: field=%s message=%s\n", validationErr.Field, validationErr.Message)
	}
}

func main() {
	repo := NewProductRepository()
	service := NewOrderService(repo)

	_, err := service.CreateOrder(1, 2)
	report("valid order", err)

	_, err = service.CreateOrder(1, 0)
	report("invalid quantity", err)

	_, err = service.CreateOrder(99, 1)
	report("missing product", err)

	_, err = service.CreateOrder(2, 1)
	report("out of stock", err)

	_, err = service.CreateOrder(1, 99)
	report("too many units", err)
}
