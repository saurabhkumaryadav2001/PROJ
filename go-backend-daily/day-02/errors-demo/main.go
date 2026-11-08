// Path: go-backend-daily/day-02/errors-demo/main.go
// Run: go run ./day-02/errors-demo
package main

import (
	"errors"
	"fmt"
)

var ErrUserNotFound = errors.New("user not found")

type ValidationError struct {
	Field   string
	Message string
}

func (e ValidationError) Error() string {
	return fmt.Sprintf("%s: %s", e.Field, e.Message)
}

func findUser(id int) (string, error) {
	if id <= 0 {
		return "", ValidationError{Field: "id", Message: "must be positive"}
	}
	if id == 42 {
		return "Saurabh", nil
	}
	return "", fmt.Errorf("find user %d: %w", id, ErrUserNotFound)
}

func main() {
	for _, id := range []int{42, 0, 99} {
		name, err := findUser(id)
		if err == nil {
			fmt.Printf("Found user: %s\n", name)
			continue
		}
		fmt.Printf("Request failed: %v\n", err)

		var validationErr ValidationError
		if errors.As(err, &validationErr) {
			fmt.Printf("Bad field=%s; reason=%s\n", validationErr.Field, validationErr.Message)
		}
		if errors.Is(err, ErrUserNotFound) {
			fmt.Println("Return HTTP 404 in a real API.")
		}
	}
}
