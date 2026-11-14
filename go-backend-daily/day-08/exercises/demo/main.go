package main

import (
	"fmt"

	"github.com/saurabhkumaryadav2001/go-backend-daily/day-08/exercises/internal/users"
)

func main() {
	user, err := users.NewUser(42, " Saurabh ")
	if err != nil {
		fmt.Println("error:", err)
		return
	}
	fmt.Printf("user: %+v\n", user)

	_, err = users.NewUser(0, "Saurabh")
	fmt.Println("invalid user:", err)
}
