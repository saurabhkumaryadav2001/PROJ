// Path: go-backend-daily/day-02/functions/main.go
// Run from go-backend-daily: go run ./day-02/functions
package main

import (
	"errors"
	"fmt"
	"strings"
)

var ErrEmptyOrder = errors.New("order must contain at least one item")

type Item struct {
	Name      string
	UnitPrice float64
	Quantity  int
}

func main() {
	fmt.Println(greet("Saurabh"))

	items := []Item{
		{Name: "Keyboard", UnitPrice: 1200, Quantity: 1},
		{Name: "Mouse", UnitPrice: 500, Quantity: 2},
	}
	total, err := calculateTotal(items, 10)
	if err != nil {
		fmt.Printf("Could not calculate order: %v\n", err)
		return
	}
	fmt.Printf("Order total after discount: ₹%.2f\n", total)

	_, err = calculateTotal(nil, 10)
	if err != nil {
		fmt.Printf("Expected failure: %v\n", err)
		if errors.Is(err, ErrEmptyOrder) {
			fmt.Println("Caller recognized the empty-order condition.")
		}
	}
}

func greet(name string) string {
	return fmt.Sprintf("Hello, %s! Welcome to Go backend learning.", name)
}

func calculateSubtotal(items []Item) (float64, error) {
	if len(items) == 0 {
		return 0, ErrEmptyOrder
	}
	subtotal := 0.0
	for _, item := range items {
		if strings.TrimSpace(item.Name) == "" {
			return 0, errors.New("item name cannot be empty")
		}
		if item.UnitPrice <= 0 {
			return 0, fmt.Errorf("%s: unit price must be greater than zero", item.Name)
		}
		if item.Quantity <= 0 {
			return 0, fmt.Errorf("%s: quantity must be greater than zero", item.Name)
		}
		subtotal += item.UnitPrice * float64(item.Quantity)
	}
	return subtotal, nil
}

func applyDiscount(subtotal, percent float64) (float64, error) {
	if percent < 0 || percent > 100 {
		return 0, fmt.Errorf("discount %.2f%% is outside the allowed range 0–100", percent)
	}
	discount := subtotal * percent / 100
	return subtotal - discount, nil
}

func calculateTotal(items []Item, discountPercent float64) (float64, error) {
	subtotal, err := calculateSubtotal(items)
	if err != nil {
		return 0, fmt.Errorf("calculate subtotal: %w", err)
	}
	total, err := applyDiscount(subtotal, discountPercent)
	if err != nil {
		return 0, fmt.Errorf("apply discount: %w", err)
	}
	return total, nil
}
