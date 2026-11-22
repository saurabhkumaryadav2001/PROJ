package main

import (
	"context"
	"errors"
	"fmt"
	"net"
	"net/http"
	"os"
	"os/signal"
	"strconv"
	"sync/atomic"
	"syscall"
	"time"
)

func main() {
	if err := run(); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}

func run() error {
	cfg, err := LoadConfig()
	if err != nil {
		return fmt.Errorf("configuration error: %w", err)
	}

	if len(os.Args) > 1 && os.Args[1] == "healthcheck" {
		return checkHealth(healthcheckURL(cfg.Port))
	}

	logger := NewLogger(cfg)
	ready := &atomic.Bool{}
	startedAt := time.Now()

	server := &http.Server{
		Addr:              ":" + strconv.Itoa(cfg.Port),
		Handler:           NewHandler(cfg, logger, ready, startedAt),
		ReadHeaderTimeout: cfg.ReadHeaderTimeout,
		ReadTimeout:       cfg.ReadTimeout,
		WriteTimeout:      cfg.WriteTimeout,
		IdleTimeout:       cfg.IdleTimeout,
	}

	stopContext, stop := signal.NotifyContext(
		context.Background(),
		os.Interrupt,
		syscall.SIGTERM,
	)
	defer stop()

	listener, err := net.Listen("tcp", server.Addr)
	if err != nil {
		return fmt.Errorf("listen on %s: %w", server.Addr, err)
	}

	// Mark ready only after the TCP listener has been created successfully.
	ready.Store(true)
	serverErrors := make(chan error, 1)
	go func() {
		logger.Info("server_starting",
			"address", server.Addr,
			"app_env", cfg.AppEnv,
			"version", version,
		)
		serverErrors <- server.Serve(listener)
	}()

	select {
	case err := <-serverErrors:
		if errors.Is(err, http.ErrServerClosed) {
			return nil
		}
		return fmt.Errorf("HTTP server failed: %w", err)

	case <-stopContext.Done():
		ready.Store(false)
		logger.Info("shutdown_started")

		shutdownContext, cancel := context.WithTimeout(
			context.Background(),
			cfg.ShutdownTimeout,
		)
		defer cancel()

		if err := server.Shutdown(shutdownContext); err != nil {
			logger.Error("graceful_shutdown_failed", "error", err)
			_ = server.Close()
			return fmt.Errorf("graceful shutdown failed: %w", err)
		}

		if err := <-serverErrors; err != nil && !errors.Is(err, http.ErrServerClosed) {
			return fmt.Errorf("HTTP server stopped unexpectedly: %w", err)
		}

		logger.Info("server_stopped")
		return nil
	}
}
