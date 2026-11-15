package main

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"time"
)

// Product is the domain model. JSON tags define the external field names.
type Product struct {
	ID        int       `json:"id"`
	SKU       string    `json:"sku"`
	Name      string    `json:"name"`
	Price     float64   `json:"price"`
	Tags      []string  `json:"tags,omitempty"`
	CreatedAt time.Time `json:"created_at"`
}

// CreateProductRequest is an API input DTO, not the database/domain model.
type CreateProductRequest struct {
	SKU   string   `json:"sku"`
	Name  string   `json:"name"`
	Price float64  `json:"price"`
	Tags  []string `json:"tags,omitempty"`
}

func (req *CreateProductRequest) Validate() error {
	req.SKU = strings.TrimSpace(req.SKU)
	req.Name = strings.TrimSpace(req.Name)

	if req.SKU == "" {
		return errors.New("sku is required")
	}
	if req.Name == "" {
		return errors.New("name is required")
	}
	if req.Price <= 0 {
		return errors.New("price must be greater than zero")
	}
	return nil
}

// decodeOneJSON accepts exactly one JSON value and rejects unknown fields.
func decodeOneJSON(r io.Reader, dst any) error {
	decoder := json.NewDecoder(r)
	decoder.DisallowUnknownFields()

	if err := decoder.Decode(dst); err != nil {
		return fmt.Errorf("decode JSON: %w", err)
	}

	// A second Decode must return EOF. Otherwise the body contains another value.
	if err := decoder.Decode(new(any)); err != io.EOF {
		if err == nil {
			return errors.New("body must contain only one JSON value")
		}
		return fmt.Errorf("check trailing JSON: %w", err)
	}
	return nil
}

func readProducts(path string) ([]Product, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("read products file: %w", err)
	}

	var products []Product
	if err := json.Unmarshal(data, &products); err != nil {
		return nil, fmt.Errorf("parse products file: %w", err)
	}
	return products, nil
}

// writeProductsAtomic writes a temporary file in the same directory, then renames it.
// This prevents readers from observing a partially-written JSON file.
func writeProductsAtomic(path string, products []Product) error {
	data, err := json.MarshalIndent(products, "", "  ")
	if err != nil {
		return fmt.Errorf("encode products: %w", err)
	}
	data = append(data, '\n')

	dir := filepath.Dir(path)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return fmt.Errorf("create directory: %w", err)
	}

	temp, err := os.CreateTemp(dir, ".products-*.tmp")
	if err != nil {
		return fmt.Errorf("create temporary file: %w", err)
	}
	tempName := temp.Name()
	defer os.Remove(tempName)

	if _, err := temp.Write(data); err != nil {
		_ = temp.Close()
		return fmt.Errorf("write temporary file: %w", err)
	}
	if err := temp.Sync(); err != nil {
		_ = temp.Close()
		return fmt.Errorf("sync temporary file: %w", err)
	}
	if err := temp.Close(); err != nil {
		return fmt.Errorf("close temporary file: %w", err)
	}
	if err := os.Rename(tempName, path); err != nil {
		return fmt.Errorf("replace products file: %w", err)
	}
	return nil
}

func main() {
	const dataPath = "day-09/jsonio/data/products.json"

	products, err := readProducts(dataPath)
	if err != nil {
		if !errors.Is(err, os.ErrNotExist) {
			fmt.Println("Initial read:", err)
			return
		}
		products = []Product{
			{ID: 1, SKU: "KB-01", Name: "Keyboard", Price: 1499, Tags: []string{"input", "office"}, CreatedAt: time.Date(2026, 1, 10, 9, 0, 0, 0, time.UTC)},
			{ID: 2, SKU: "MS-01", Name: "Mouse", Price: 799, CreatedAt: time.Date(2026, 1, 11, 9, 0, 0, 0, time.UTC)},
		}
		if writeErr := writeProductsAtomic(dataPath, products); writeErr != nil {
			fmt.Println("Seed write:", writeErr)
			return
		}
	}

	// Simulate an incoming HTTP JSON body.
	body := `{"sku":"HD-01","name":"Headphones","price":2499,"tags":["audio"]}`
	var request CreateProductRequest
	if err := decodeOneJSON(strings.NewReader(body), &request); err != nil {
		fmt.Println("Request decode:", err)
		return
	}
	if err := request.Validate(); err != nil {
		fmt.Println("Request validation:", err)
		return
	}

	nextID := len(products) + 1
	products = append(products, Product{
		ID: nextID, SKU: request.SKU, Name: request.Name, Price: request.Price,
		Tags: request.Tags, CreatedAt: time.Now().UTC(),
	})

	if err := writeProductsAtomic(dataPath, products); err != nil {
		fmt.Println("Save:", err)
		return
	}

	// JSON response: MarshalIndent is useful for readable CLI output.
	response, err := json.MarshalIndent(products, "", "  ")
	if err != nil {
		fmt.Println("Response encode:", err)
		return
	}
	fmt.Println(string(response))
}
