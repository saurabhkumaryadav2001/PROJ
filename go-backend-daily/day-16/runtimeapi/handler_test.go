package main

import (
	"encoding/json"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"
	"time"
)

func testConfig() Config {
	cfg, err := loadConfig(mapEnv(nil))
	if err != nil {
		panic(err)
	}
	return cfg
}

func TestHealthAndReadiness(t *testing.T) {
	ready := &atomic.Bool{}
	handler := NewHandler(testConfig(), slog.New(slog.NewTextHandler(io.Discard, nil)), ready, time.Now())

	health := httptest.NewRecorder()
	handler.ServeHTTP(health, httptest.NewRequest(http.MethodGet, "/healthz", nil))
	if health.Code != http.StatusOK {
		t.Fatalf("health status = %d, want 200", health.Code)
	}

	notReady := httptest.NewRecorder()
	handler.ServeHTTP(notReady, httptest.NewRequest(http.MethodGet, "/readyz", nil))
	if notReady.Code != http.StatusServiceUnavailable {
		t.Fatalf("readiness status = %d, want 503", notReady.Code)
	}

	ready.Store(true)
	isReady := httptest.NewRecorder()
	handler.ServeHTTP(isReady, httptest.NewRequest(http.MethodGet, "/readyz", nil))
	if isReady.Code != http.StatusOK {
		t.Fatalf("readiness status = %d, want 200", isReady.Code)
	}
}

func TestRuntimeEndpointReturnsSafeMetadata(t *testing.T) {
	ready := &atomic.Bool{}
	cfg := testConfig()
	cfg.AppEnv = "production"
	handler := NewHandler(cfg, slog.New(slog.NewTextHandler(io.Discard, nil)), ready, time.Now())

	recorder := httptest.NewRecorder()
	handler.ServeHTTP(recorder, httptest.NewRequest(http.MethodGet, "/api/v1/runtime", nil))

	if recorder.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200", recorder.Code)
	}
	var body map[string]any
	if err := json.Unmarshal(recorder.Body.Bytes(), &body); err != nil {
		t.Fatalf("invalid JSON: %v", err)
	}
	if body["app_env"] != "production" || body["go_version"] == "" {
		t.Fatalf("unexpected runtime response: %v", body)
	}
	if strings.Contains(recorder.Body.String(), "DATABASE_URL") ||
		strings.Contains(recorder.Body.String(), "password") {
		t.Fatalf("runtime endpoint exposed sensitive configuration: %s", recorder.Body.String())
	}
}

func TestRequestLoggingMiddlewareAddsRequestID(t *testing.T) {
	ready := &atomic.Bool{}
	handler := NewHandler(testConfig(), slog.New(slog.NewTextHandler(io.Discard, nil)), ready, time.Now())

	recorder := httptest.NewRecorder()
	handler.ServeHTTP(recorder, httptest.NewRequest(http.MethodGet, "/healthz", nil))

	requestID := recorder.Header().Get("X-Request-ID")
	if len(requestID) != 32 {
		t.Fatalf("request ID length = %d, want 32", len(requestID))
	}
}
