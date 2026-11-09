// Path: go-backend-daily/day-03/exercises/solutions.go
// Run from go-backend-daily: go run ./day-03/exercises
package main

import (
	"fmt"
	"strings"
)

func classifyTemperature(celsius float64) string {
	if celsius < 0 { return "freezing" }
	if celsius < 20 { return "cold" }
	if celsius <= 30 { return "comfortable" }
	return "hot"
}

func countPositive(numbers []int) int {
	count := 0
	for _, number := range numbers {
		if number <= 0 { continue }
		count++
	}
	return count
}

func findFirstEven(numbers []int) (int, bool) {
	for _, number := range numbers {
		if number%2 == 0 { return number, true }
	}
	return 0, false
}

func normalizeRole(role string) string {
	switch strings.ToLower(strings.TrimSpace(role)) {
	case "admin", "administrator": return "ADMIN"
	case "driver": return "DRIVER"
	case "maintenance", "mechanic": return "MAINTENANCE"
	default: return "UNKNOWN"
	}
}

func main() {
	fmt.Println("temperature 24:", classifyTemperature(24))
	fmt.Println("positive count:", countPositive([]int{-2, 0, 4, 7, -1, 9}))
	value, ok := findFirstEven([]int{3, 5, 8, 11})
	if ok { fmt.Println("first even:", value) } else { fmt.Println("no even number") }
	fmt.Println("role:", normalizeRole("  Admin "))
}
