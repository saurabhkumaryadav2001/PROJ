package catalog

import (
	"errors"
	"strings"
)

var ErrInvalidProduct = errors.New("invalid product")

// Product is the catalog's public data model.
type Product struct {
	ID   int    `json:"id"`
	SKU  string `json:"sku"`
	Name string `json:"name"`
}

// NewProduct validates input and returns a ready-to-use Product.
func NewProduct(id int, sku, name string) (Product, error) {
	sku = normalizeSKU(sku)
	name = strings.TrimSpace(name)

	if id <= 0 || !validateSKU(sku) || name == "" {
		return Product{}, ErrInvalidProduct
	}

	return Product{ID: id, SKU: sku, Name: name}, nil
}

// normalizeSKU is intentionally private to this package.
func normalizeSKU(sku string) string {
	return strings.ToUpper(strings.TrimSpace(sku))
}

func validateSKU(sku string) bool {
	return len(sku) >= 3
}

// List returns a slice copy for callers.
func List() []Product {
	return []Product{
		{ID: 1, SKU: "KB-01", Name: "Keyboard"},
		{ID: 2, SKU: "MS-01", Name: "Mouse"},
	}
}
