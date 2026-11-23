package main

import (
	"fmt"
	"os"
	"strconv"
	"strings"
	"time"
)

type Config struct {
	Port            int
	Version         string
	ReadHeaderLimit time.Duration
	ReadTimeout     time.Duration
	WriteTimeout    time.Duration
	IdleTimeout     time.Duration
	ShutdownTimeout time.Duration
}

func LoadConfig() (Config, error) {
	return loadConfig(os.Getenv)
}

func loadConfig(getenv func(string) string) (Config, error) {
	portText := valueOr(getenv, "PORT", "8080")
	port, err := strconv.Atoi(portText)
	if err != nil || port < 1 || port > 65535 {
		return Config{}, fmt.Errorf("PORT must be an integer from 1 to 65535")
	}

	cfg := Config{Port: port, Version: valueOr(getenv, "APP_VERSION", "dev")}
	if cfg.ReadHeaderLimit, err = readDuration(getenv, "READ_HEADER_TIMEOUT", "5s"); err != nil { return Config{}, err }
	if cfg.ReadTimeout, err = readDuration(getenv, "READ_TIMEOUT", "10s"); err != nil { return Config{}, err }
	if cfg.WriteTimeout, err = readDuration(getenv, "WRITE_TIMEOUT", "15s"); err != nil { return Config{}, err }
	if cfg.IdleTimeout, err = readDuration(getenv, "IDLE_TIMEOUT", "60s"); err != nil { return Config{}, err }
	if cfg.ShutdownTimeout, err = readDuration(getenv, "SHUTDOWN_TIMEOUT", "10s"); err != nil { return Config{}, err }
	return cfg, nil
}

func valueOr(getenv func(string) string, key, fallback string) string {
	if value := strings.TrimSpace(getenv(key)); value != "" { return value }
	return fallback
}

func readDuration(getenv func(string) string, key, fallback string) (time.Duration, error) {
	value, err := time.ParseDuration(valueOr(getenv, key, fallback))
	if err != nil || value <= 0 { return 0, fmt.Errorf("%s must be a positive duration such as 5s", key) }
	return value, nil
}
