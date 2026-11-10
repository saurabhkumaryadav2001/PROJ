// Path: go-backend-daily/day-04/exercises/solutions.go
// Run from go-backend-daily: go run ./day-04/exercises
package main

import (
	"fmt"
	"strings"
)

func average(numbers []int) (float64, bool) {
	if len(numbers) == 0 { return 0, false }
	sum := 0
	for _, n := range numbers { sum += n }
	return float64(sum) / float64(len(numbers)), true
}

func removeAt(values []string, index int) ([]string, bool) {
	if index < 0 || index >= len(values) { return values, false }
	copy(values[index:], values[index+1:])
	values = values[:len(values)-1]
	return values, true
}

func frequency(words []string) map[string]int {
	counts := make(map[string]int)
	for _, word := range words {
		word = strings.ToLower(strings.TrimSpace(word))
		if word == "" { continue }
		counts[word]++
	}
	return counts
}

func mergeStock(first, second map[string]int) map[string]int {
	merged := make(map[string]int, len(first)+len(second))
	for sku, qty := range first { merged[sku] = qty }
	for sku, qty := range second { merged[sku] += qty }
	return merged
}

func main() {
	avg, ok := average([]int{10, 20, 30})
	fmt.Println("average:", avg, ok)
	values, removed := removeAt([]string{"A", "B", "C"}, 1)
	fmt.Println("remove:", values, removed)
	fmt.Println("frequency:", frequency([]string{"Go", "go", " API ", ""}))
	fmt.Println("merged stock:", mergeStock(
		map[string]int{"A": 5, "B": 2},
		map[string]int{"A": 3, "C": 9},
	))
}
