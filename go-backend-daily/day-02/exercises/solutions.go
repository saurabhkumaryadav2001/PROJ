// Path: go-backend-daily/day-02/exercises/solutions.go
// Run: go run ./day-02/exercises
package main

import (
	"errors"
	"fmt"
)

func max(a, b int) int {
	if a > b { return a }
	return b
}

func safeDivide(a, b float64) (float64, error) {
	if b == 0 { return 0, errors.New("division by zero") }
	return a / b, nil
}

func priceAfterDiscount(price, percent float64) (float64, error) {
	if price < 0 { return 0, errors.New("price cannot be negative") }
	if percent < 0 || percent > 100 { return 0, errors.New("discount must be between 0 and 100") }
	return price * (1 - percent/100), nil
}

func main() {
	fmt.Println("max:", max(12, 7))
	value, err := safeDivide(20, 4)
	if err != nil { fmt.Println("divide error:", err); return }
	fmt.Println("20 / 4 =", value)
	price, err := priceAfterDiscount(1000, 15)
	if err != nil { fmt.Println("discount error:", err); return }
	fmt.Printf("price after 15%% discount = %.2f\n", price)
}
