package main

import (
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"log/slog"
	"net/http"
	"runtime"
	"sync/atomic"
	"time"
)

type API struct {
	config    Config
	logger    *slog.Logger
	ready     *atomic.Bool
	startedAt time.Time
}

type runtimeInfo struct {
	Status        string  `json:"status"`
	AppEnv        string  `json:"app_env"`
	Version       string  `json:"version"`
	GoVersion     string  `json:"go_version"`
	UptimeSeconds float64 `json:"uptime_seconds"`
}

var version = "dev"

func NewHandler(cfg Config, logger *slog.Logger, ready *atomic.Bool, startedAt time.Time) http.Handler {
	api := &API{
		config:    cfg,
		logger:    logger,
		ready:     ready,
		startedAt: startedAt,
	}

	mux := http.NewServeMux()
	mux.HandleFunc("GET /healthz", api.health)
	mux.HandleFunc("GET /readyz", api.readiness)
	mux.HandleFunc("GET /api/v1/runtime", api.runtime)

	return requestLoggingMiddleware(logger, mux)
}

// health is a liveness probe: it answers whether the process can serve HTTP.
func (a *API) health(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, http.StatusOK, map[string]string{"status": "ok"})
}

// readiness is a readiness probe: it answers whether this instance should
// receive traffic. The process marks itself ready after the listener starts.
func (a *API) readiness(w http.ResponseWriter, r *http.Request) {
	if !a.ready.Load() {
		writeJSON(w, http.StatusServiceUnavailable, map[string]string{"status": "not_ready"})
		return
	}
	writeJSON(w, http.StatusOK, map[string]string{"status": "ready"})
}

// runtime returns safe operational metadata. It deliberately does not expose
// environment variables, connection strings, tokens, or other secrets.
func (a *API) runtime(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, http.StatusOK, runtimeInfo{
		Status:        "ok",
		AppEnv:        a.config.AppEnv,
		Version:       version,
		GoVersion:     runtime.Version(),
		UptimeSeconds: time.Since(a.startedAt).Seconds(),
	})
}

func requestLoggingMiddleware(logger *slog.Logger, next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requestID := newRequestID()
		w.Header().Set("X-Request-ID", requestID)

		recorder := &statusRecorder{ResponseWriter: w, status: http.StatusOK}
		started := time.Now()
		next.ServeHTTP(recorder, r)

		logger.Info("http_request",
			"request_id", requestID,
			"method", r.Method,
			"path", r.URL.Path,
			"status", recorder.status,
			"duration_ms", time.Since(started).Milliseconds(),
		)
	})
}

type statusRecorder struct {
	http.ResponseWriter
	status int
}

func (w *statusRecorder) WriteHeader(status int) {
	w.status = status
	w.ResponseWriter.WriteHeader(status)
}

func (w *statusRecorder) Write(data []byte) (int, error) {
	return w.ResponseWriter.Write(data)
}

func newRequestID() string {
	var value [16]byte
	if _, err := rand.Read(value[:]); err != nil {
		return "request-id-unavailable"
	}
	return hex.EncodeToString(value[:])
}

func writeJSON(w http.ResponseWriter, status int, value any) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(value)
}
