package main

import (
	"log/slog"
	"os"
)

// NewLogger creates structured JSON logs suitable for container log collectors.
func NewLogger(cfg Config) *slog.Logger {
	handler := slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{
		Level: cfg.LogLevel,
	})
	return slog.New(handler)
}
