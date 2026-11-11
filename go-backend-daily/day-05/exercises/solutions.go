package main

import (
	"errors"
	"fmt"
	"strings"
)

type Product struct {
	SKU   string
	Name  string
	Price float64
	Stock int
}

func (p Product) InventoryValue() float64 {
	return p.Price * float64(p.Stock)
}

func (p *Product) Restock(quantity int) error {
	if quantity <= 0 {
		return errors.New("restock quantity must be positive")
	}
	p.Stock += quantity
	return nil
}

func (p *Product) Rename(name string) error {
	name = strings.TrimSpace(name)
	if name == "" {
		return errors.New("product name cannot be blank")
	}
	p.Name = name
	return nil
}

func main() {
	product := Product{SKU: "SKU-101", Name: "Box", Price: 12.5, Stock: 20}
	fmt.Println("inventory value:", product.InventoryValue())

	if err := product.Restock(10); err != nil {
		fmt.Println("restock error:", err)
	}
	if err := product.Rename("Premium Box"); err != nil {
		fmt.Println("rename error:", err)
	}

	fmt.Printf("updated product=%+v\n", product)
	fmt.Println("updated value:", product.InventoryValue())
}
