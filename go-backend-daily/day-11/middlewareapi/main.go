package main

import (
	"log"
	"net/http"
	"os"
	"time"
)

func main() {
	logger := log.New(os.Stdout, "api ", log.LstdFlags|log.LUTC)
	store := NewProductStore()
	api := NewAPI(store, logger)

	mux := http.NewServeMux()
	mux.HandleFunc("GET /healthz", api.health)
	mux.HandleFunc("GET /api/v1/products", api.listProducts)

	// This route is protected by Bearer authentication.
	// API_TOKEN is read from the environment; the fallback is only for local learning.
	token := os.Getenv("API_TOKEN")
	if token == "" {
		token = "dev-token"
		logger.Println("WARNING: using the demo API token; set API_TOKEN outside local learning")
	}
	mux.Handle("POST /api/v1/products", BearerAuth(token)(http.HandlerFunc(api.createProduct)))
	mux.Handle("GET /api/v1/private", BearerAuth(token)(http.HandlerFunc(api.privateInfo)))

	// Wrapping order: RequestID -> Logging -> Recover -> SecurityHeaders -> routes.
	handler := Chain(
		mux,
		RequestID,
		Logging(logger),
		Recover(logger),
		SecurityHeaders,
	)

	port := os.Getenv("PORT")
	if port == "" {
		port = "8080"
	}
	server := &http.Server{
		Addr:              ":" + port,
		Handler:           handler,
		ReadHeaderTimeout: 5 * time.Second,
		ReadTimeout:       10 * time.Second,
		WriteTimeout:      15 * time.Second,
		IdleTimeout:       60 * time.Second,
	}

	logger.Printf("listening on :%s", port)
	if err := server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
		logger.Fatalf("server failed: %v", err)
	}
}
