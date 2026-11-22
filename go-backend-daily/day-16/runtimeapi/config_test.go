package main

import (
	"testing"
	"time"
)

func mapEnv(values map[string]string) func(string) string {
	return func(key string) string { return values[key] }
}

func TestLoadConfigDefaults(t *testing.T) {
	cfg, err := loadConfig(mapEnv(nil))
	if err != nil {
		t.Fatalf("loadConfig returned error: %v", err)
	}

	if cfg.AppEnv != "development" || cfg.Port != 8080 {
		t.Fatalf("unexpected defaults: %+v", cfg)
	}
	if cfg.ReadHeaderTimeout != 5*time.Second || cfg.ShutdownTimeout != 10*time.Second {
		t.Fatalf("unexpected timeout defaults: %+v", cfg)
	}
}

func TestLoadConfigOverrides(t *testing.T) {
	cfg, err := loadConfig(mapEnv(map[string]string{
		"APP_ENV":             "production",
		"PORT":                "9090",
		"LOG_LEVEL":           "debug",
		"READ_HEADER_TIMEOUT": "2s",
		"READ_TIMEOUT":        "8s",
		"WRITE_TIMEOUT":       "12s",
		"IDLE_TIMEOUT":        "30s",
		"SHUTDOWN_TIMEOUT":    "7s",
	}))
	if err != nil {
		t.Fatalf("loadConfig returned error: %v", err)
	}

	if cfg.AppEnv != "production" || cfg.Port != 9090 || cfg.LogLevelName != "debug" {
		t.Fatalf("overrides not applied: %+v", cfg)
	}
	if cfg.ReadHeaderTimeout != 2*time.Second || cfg.ShutdownTimeout != 7*time.Second {
		t.Fatalf("duration overrides not applied: %+v", cfg)
	}
}

func TestLoadConfigRejectsInvalidValues(t *testing.T) {
	cases := []struct {
		name string
		env  map[string]string
	}{
		{name: "unknown environment", env: map[string]string{"APP_ENV": "prod"}},
		{name: "non numeric port", env: map[string]string{"PORT": "eight"}},
		{name: "port out of range", env: map[string]string{"PORT": "70000"}},
		{name: "invalid log level", env: map[string]string{"LOG_LEVEL": "verbose"}},
		{name: "invalid duration", env: map[string]string{"READ_TIMEOUT": "soon"}},
		{name: "zero duration", env: map[string]string{"IDLE_TIMEOUT": "0s"}},
	}

	for _, testCase := range cases {
		t.Run(testCase.name, func(t *testing.T) {
			if _, err := loadConfig(mapEnv(testCase.env)); err == nil {
				t.Fatal("expected configuration error")
			}
		})
	}
}
