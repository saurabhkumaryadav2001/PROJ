package main

import (
	"context"
	"errors"
	"log"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"
)

func main() {
	logger := log.New(os.Stdout, "postgres-api ", log.LstdFlags|log.LUTC)
	databaseURL := os.Getenv("DATABASE_URL")
	if databaseURL == "" {
		logger.Fatal("DATABASE_URL is required; see README.md for local PostgreSQL setup")
	}

	startupCtx, cancelStartup := context.WithTimeout(context.Background(), 5*time.Second)
	db, err := OpenDB(startupCtx, databaseURL)
	cancelStartup()
	if err != nil {
		logger.Fatalf("database startup failed: %v", err)
	}
	defer db.Close()

	api := NewAPI(NewSQLRepository(db))
	port := os.Getenv("PORT")
	if port == "" {
		port = "8080"
	}

	server := &http.Server{
		Addr:              ":" + port,
		Handler:           api.Handler(),
		ReadHeaderTimeout: 5 * time.Second,
		ReadTimeout:       10 * time.Second,
		WriteTimeout:      15 * time.Second,
		IdleTimeout:       60 * time.Second,
	}

	serverErrors := make(chan error, 1)
	go func() {
		logger.Printf("API listening on :%s", port)
		serverErrors <- server.ListenAndServe()
	}()

	stopCtx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	select {
	case err := <-serverErrors:
		if !errors.Is(err, http.ErrServerClosed) {
			logger.Printf("HTTP server failed: %v", err)
			return // allow deferred database cleanup to run
		}
	case <-stopCtx.Done():
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
