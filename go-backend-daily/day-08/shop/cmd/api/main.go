package main

import (
	"log"
	"net/http"

	"github.com/saurabhkumaryadav2001/go-backend-daily/day-08/shop/internal/catalog"
	"github.com/saurabhkumaryadav2001/go-backend-daily/day-08/shop/internal/httpapi"
)

func main() {
	handler := httpapi.NewHandler(catalog.List())

	log.Println("API listening on :8080")
	if err := http.ListenAndServe(":8080", handler); err != nil {
		log.Fatal(err)
	}
}
