package main

import (
	"context"
	"errors"
	"log"
	"net/http"
	"os"
	"os/signal"
	"strconv"
	"syscall"
	"time"
)

func main() {
	logger := log.New(os.Stdout, "concurrency-api ", log.LstdFlags|log.LUTC)
	workers := envInt("WORKERS", 4)
	queueSize := envInt("QUEUE_SIZE", workers*2)

	quoteService, err := NewQuoteService(workers, queueSize)
	if err != nil {
		logger.Fatalf("configure worker pool: %v", err)
	}

	api := NewAPI(quoteService, &Metrics{})
	server := &http.Server{
		Addr:              ":" + envString("PORT", "8080"),
		Handler:           api.Handler(),
		ReadHeaderTimeout: 5 * time.Second,
		ReadTimeout:       10 * time.Second,
		WriteTimeout:      15 * time.Second,
		IdleTimeout:       60 * time.Second,
	}

	stopCtx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	serverErrors := make(chan error, 1)
	go func() {
		logger.Printf("listening on %s with %d workers and queue capacity %d", server.Addr, workers, queueSize)
		serverErrors <- server.ListenAndServe()
	}()

	select {
	case err := <-serverErrors:
		if !errors.Is(err, http.ErrServerClosed) {
			logger.Fatalf("HTTP server failed: %v", err)
		}
	case <-stopCtx.Done():
		logger.Println("shutdown signal received")
		shutdownCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()
		if err := server.Shutdown(shutdownCtx); err != nil {
			logger.Printf("graceful shutdown failed: %v", err)
			_ = server.Close()
		}
		if err := <-serverErrors; err != nil && !errors.Is(err, http.ErrServerClosed) {
			logger.Printf("HTTP server stopped with error: %v", err)
		}
	}
}

func envString(key, fallback string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return fallback
}

func envInt(key string, fallback int) int {
	value := os.Getenv(key)
	if value == "" {
		return fallback
	}
	parsed, err := strconv.Atoi(value)
	if err != nil {
		return fallback
	}
	return parsed
}
