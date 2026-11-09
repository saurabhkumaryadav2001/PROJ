// Path: go-backend-daily/day-03/control-flow/main.go
// Run from go-backend-daily: go run ./day-03/control-flow
package main

import (
	"fmt"
	"strings"
)

type Shipment struct {
	ID          int
	Destination string
	Status      string
	WeightKg    float64
}

func main() {
	shipments := []Shipment{
		{ID: 101, Destination: "Mumbai", Status: "pending", WeightKg: 12.5},
		{ID: 102, Destination: "Pune", Status: "in_transit", WeightKg: 8},
		{ID: 103, Destination: "Hyderabad", Status: "delivered", WeightKg: 5},
		{ID: 104, Destination: "  ", Status: "pending", WeightKg: 4},
		{ID: 105, Destination: "Bengaluru", Status: "unknown", WeightKg: 7},
		{ID: 106, Destination: "Delhi", Status: "pending", WeightKg: -2},
	}

	fmt.Println("=== Shipment processing ===")
	processed, rejected := processShipments(shipments)
	fmt.Printf("Summary: processed=%d rejected=%d total=%d\n\n",
		processed, rejected, len(shipments))

	fmt.Println("=== Classic for loop ===")
	for attempt := 1; attempt <= 3; attempt++ {
		if attempt == 2 {
			fmt.Println("Attempt 2: temporary failure; skip this attempt")
			continue // jump to the next iteration
		}
		fmt.Printf("Attempt %d: running\n", attempt)
		if attempt == 3 {
			fmt.Println("Attempt limit reached; stop")
			break // exit the loop completely
		}
	}

	fmt.Println("\n=== Find shipment with early return ===")
	found, ok := findShipment(shipments, 102)
	if !ok {
		fmt.Println("Shipment not found")
		return
	}
	fmt.Printf("Found shipment: %+v\n", found)
}

// processShipments demonstrates range, guard clauses, if/else and switch.
// It returns counts so callers can make decisions without parsing printed text.
func processShipments(shipments []Shipment) (processed int, rejected int) {
	for _, shipment := range shipments {
		// Guard clause: invalid destination should not reach business logic.
		if strings.TrimSpace(shipment.Destination) == "" {
			fmt.Printf("Shipment %d rejected: destination is required\n", shipment.ID)
			rejected++
			continue
		}

		// Normalize user/database text before matching.
		status := strings.ToLower(strings.TrimSpace(shipment.Status))

		switch status {
		case "pending":
			if shipment.WeightKg <= 0 {
				fmt.Printf("Shipment %d rejected: weight must be positive\n", shipment.ID)
				rejected++
				continue
			}
			fmt.Printf("Shipment %d (%s): ready to dispatch\n",
				shipment.ID, shipment.Destination)
			processed++

		case "in_transit":
			fmt.Printf("Shipment %d: already in transit; monitor it\n", shipment.ID)
			processed++

		case "delivered", "cancelled":
			// Multiple values can share a case.
			fmt.Printf("Shipment %d: terminal status %s; no action\n",
				shipment.ID, status)

		default:
			fmt.Printf("Shipment %d rejected: unsupported status %q\n",
				shipment.ID, status)
			rejected++
		}
	}
	return processed, rejected
}

// findShipment uses an index-based loop and returns early when it finds a match.
func findShipment(shipments []Shipment, targetID int) (Shipment, bool) {
	for i := 0; i < len(shipments); i++ {
		if shipments[i].ID == targetID {
			return shipments[i], true
		}
	}
	return Shipment{}, false
}
