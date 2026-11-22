package main

import (
	"fmt"
	"log/slog"
	"os"
	"strconv"
	"strings"
	"time"
)

// Config contains non-secret runtime settings. Secrets should come from a
// dedicated secret manager or orchestrator secret mount, not source control.
type Config struct {
	AppEnv            string
	Port              int
	LogLevel          slog.Level
	LogLevelName      string
	ReadHeaderTimeout time.Duration
	ReadTimeout       time.Duration
	WriteTimeout      time.Duration
	IdleTimeout       time.Duration
	ShutdownTimeout   time.Duration
}

// LoadConfig reads and validates configuration from the process environment.
func LoadConfig() (Config, error) {
	return loadConfig(os.Getenv)
}

func loadConfig(getenv func(string) string) (Config, error) {
	appEnv := valueOr(getenv, "APP_ENV", "development")
	switch appEnv {
	case "development", "staging", "production":
	default:
		return Config{}, fmt.Errorf("APP_ENV must be development, staging, or production")
	}

	portText := valueOr(getenv, "PORT", "8080")
	port, err := strconv.Atoi(portText)
	if err != nil || port < 1 || port > 65535 {
		return Config{}, fmt.Errorf("PORT must be an integer between 1 and 65535")
	}

	levelName := strings.ToLower(valueOr(getenv, "LOG_LEVEL", "info"))
	var level slog.Level
	switch levelName {
	case "debug":
		level = slog.LevelDebug
	case "info":
		level = slog.LevelInfo
	case "warn":
		level = slog.LevelWarn
	case "error":
		level = slog.LevelError
	default:
		return Config{}, fmt.Errorf("LOG_LEVEL must be debug, info, warn, or error")
	}

	readHeaderTimeout, err := durationValue(getenv, "READ_HEADER_TIMEOUT", "5s")
	if err != nil {
		return Config{}, err
	}
	readTimeout, err := durationValue(getenv, "READ_TIMEOUT", "10s")
	if err != nil {
		return Config{}, err
	}
	writeTimeout, err := durationValue(getenv, "WRITE_TIMEOUT", "15s")
	if err != nil {
		return Config{}, err
	}
	idleTimeout, err := durationValue(getenv, "IDLE_TIMEOUT", "60s")
	if err != nil {
		return Config{}, err
	}
	shutdownTimeout, err := durationValue(getenv, "SHUTDOWN_TIMEOUT", "10s")
	if err != nil {
		return Config{}, err
	}

	return Config{
		AppEnv:            appEnv,
		Port:              port,
		LogLevel:          level,
		LogLevelName:      levelName,
		ReadHeaderTimeout: readHeaderTimeout,
		ReadTimeout:       readTimeout,
		WriteTimeout:      writeTimeout,
		IdleTimeout:       idleTimeout,
		ShutdownTimeout:   shutdownTimeout,
	}, nil
}

func valueOr(getenv func(string) string, key, fallback string) string {
	if value := strings.TrimSpace(getenv(key)); value != "" {
		return value
	}
	return fallback
}

func durationValue(getenv func(string) string, key, fallback string) (time.Duration, error) {
	raw := valueOr(getenv, key, fallback)
	value, err := time.ParseDuration(raw)
	if err != nil || value <= 0 {
		return 0, fmt.Errorf("%s must be a positive Go duration such as 5s or 1m", key)
	}
	return value, nil
}
