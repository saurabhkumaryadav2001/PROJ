// Path: go-backend-daily/day-04/collections/main.go
// Run from go-backend-daily: go run ./day-04/collections
package main

import (
	"fmt"
	"sort"
)

type InventoryItem struct {
	SKU      string
	Name      string
	Quantity  int
	UnitPrice float64
}

func main() {
	fmt.Println("=== 1. Array: fixed size ===")
	var weeklyLoads [3]int
	weeklyLoads[0], weeklyLoads[1], weeklyLoads[2] = 12, 18, 15
	fmt.Printf("array=%v length=%d\n\n", weeklyLoads, len(weeklyLoads))

	fmt.Println("=== 2. Slice: flexible list ===")
	shipments := []string{"SHP-101", "SHP-102"}
	shipments = append(shipments, "SHP-103")
	fmt.Printf("shipments=%v len=%d cap=%d\n\n", shipments, len(shipments), cap(shipments))

	fmt.Println("=== 3. Slices share an underlying array ===")
	original := []int{10, 20, 30, 40}
	view := original[1:3]
	view[0] = 999
	fmt.Printf("original=%v view=%v\n\n", original, view)

	fmt.Println("=== 4. copy creates independent slice storage ===")
	source := []int{1, 2, 3}
	destination := make([]int, len(source))
	copied := copy(destination, source)
	destination[0] = 100
	fmt.Printf("source=%v destination=%v copied=%d\n\n", source, destination, copied)

	fmt.Println("=== 5. Map: fast key-value lookup ===")
	stockBySKU := map[string]int{"SKU-A": 25, "SKU-B": 10}
	stockBySKU["SKU-C"] = 7
	stockBySKU["SKU-A"] -= 2
	delete(stockBySKU, "SKU-B")
	quantity, exists := stockBySKU["SKU-A"]
	fmt.Printf("SKU-A quantity=%d exists=%t map=%v\n\n", quantity, exists, stockBySKU)

	fmt.Println("=== 6. Backend inventory aggregation ===")
	items := []InventoryItem{
		{SKU: "SKU-A", Name: "Packing Box", Quantity: 25, UnitPrice: 12.5},
		{SKU: "SKU-B", Name: "Tape", Quantity: 10, UnitPrice: 30},
		{SKU: "SKU-C", Name: "Label", Quantity: 7, UnitPrice: 2},
		{SKU: "SKU-A", Name: "Packing Box", Quantity: 5, UnitPrice: 12.5},
	}
	stock, value := aggregateInventory(items)
	fmt.Println("stock by SKU:", stock)
	fmt.Printf("total inventory value=₹%.2f\n", value)

	// Map iteration order is unspecified; sort keys for stable output.
	keys := make([]string, 0, len(stock))
	for sku := range stock {
		keys = append(keys, sku)
	}
	sort.Strings(keys)
	for _, sku := range keys {
		fmt.Printf("%s -> %d units\n", sku, stock[sku])
	}

	fmt.Println("\n=== 7. Lookup with comma-ok ===")
	if qty, ok := stock["SKU-Z"]; ok {
		fmt.Println("SKU-Z stock:", qty)
	} else {
		fmt.Println("SKU-Z is absent (not the same as a stored zero)")
	}
}

func aggregateInventory(items []InventoryItem) (map[string]int, float64) {
	stock := make(map[string]int, len(items))
	totalValue := 0.0
	for _, item := range items {
		stock[item.SKU] += item.Quantity
		totalValue += float64(item.Quantity) * item.UnitPrice
	}
	return stock, totalValue
}
