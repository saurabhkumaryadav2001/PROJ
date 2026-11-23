package main

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"net"
	"net/http"
	"os"
	"os/signal"
	"sync/atomic"
	"syscall"
)

func main() {
	if err := run(); err != nil { fmt.Fprintln(os.Stderr, err); os.Exit(1) }
}
func run() error {
	cfg, err := LoadConfig()
	if err != nil { return fmt.Errorf("configuration: %w", err) }
	logger := NewLogger(os.Stdout, slog.LevelInfo)
	metrics := NewMetrics()
	ready := &atomic.Bool{}
	server := &http.Server{
		Addr: fmt.Sprintf(":%d", cfg.Port),
		Handler: NewHandler(logger, metrics, cfg.Version, ready),
		ReadHeaderTimeout: cfg.ReadHeaderLimit,
		ReadTimeout: cfg.ReadTimeout,
		WriteTimeout: cfg.WriteTimeout,
		IdleTimeout: cfg.IdleTimeout,
	}
	listener, err := net.Listen("tcp", server.Addr)
	if err != nil { return fmt.Errorf("listen: %w", err) }
	readinessAfterStartup(ready)
	stopContext, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()
	serveErrors := make(chan error, 1)
	go func() { serveErrors <- server.Serve(listener) }()
	logger.Info("server_started", "address", server.Addr, "version", cfg.Version)
	select {
	case err := <-serveErrors:
		if errors.Is(err, http.ErrServerClosed) { return nil }
		return fmt.Errorf("serve: %w", err)
	case <-stopContext.Done():
		ready.Store(false)
		shutdownContext, cancel := context.WithTimeout(context.Background(), cfg.ShutdownTimeout)
		defer cancel()
		if err := server.Shutdown(shutdownContext); err != nil { _ = server.Close(); return fmt.Errorf("shutdown: %w", err) }
		if err := <-serveErrors; err != nil && !errors.Is(err, http.ErrServerClosed) { return fmt.Errorf("server stopped: %w", err) }
		logger.Info("server_stopped")
		return nil
	}
}
