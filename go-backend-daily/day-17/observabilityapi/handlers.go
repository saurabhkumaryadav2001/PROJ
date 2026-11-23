package main

import (
	"encoding/json"
	"log/slog"
	"net/http"
	"runtime"
	"strconv"
	"sync/atomic"
)

type API struct {
	logger *slog.Logger
	metrics *Metrics
	version string
	ready *atomic.Bool
}
type runtimeResponse struct {
	Version string `json:"version"`
	GoVersion string `json:"go_version"`
}
type orderResponse struct {
	ID int64 `json:"id"`
	Status string `json:"status"`
}

func NewHandler(logger *slog.Logger, metrics *Metrics, version string, ready *atomic.Bool) http.Handler {
	api := &API{logger: logger, metrics: metrics, version: version, ready: ready}
	mux := http.NewServeMux()
	mux.HandleFunc("GET /healthz", api.health)
	mux.HandleFunc("GET /readyz", api.readiness)
	mux.HandleFunc("GET /metrics", api.prometheusMetrics)
	mux.HandleFunc("GET /api/v1/runtime", api.runtime)
	mux.HandleFunc("GET /api/v1/orders/{id}", api.getOrder)
	return ObservabilityMiddleware(metrics, logger, mux)
}
func (a *API) health(w http.ResponseWriter, r *http.Request) { writeJSON(w, http.StatusOK, map[string]string{"status":"ok"}) }
func (a *API) readiness(w http.ResponseWriter, r *http.Request) {
	if !a.ready.Load() { writeJSON(w, http.StatusServiceUnavailable, map[string]string{"status":"not_ready"}); return }
	writeJSON(w, http.StatusOK, map[string]string{"status":"ready"})
}
func (a *API) prometheusMetrics(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "text/plain; version=0.0.4; charset=utf-8")
	if err := a.metrics.WritePrometheus(w); err != nil { a.logger.ErrorContext(r.Context(), "metrics_write_failed", "error", err) }
}
func (a *API) runtime(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, http.StatusOK, runtimeResponse{Version: a.version, GoVersion: runtime.Version()})
}
func (a *API) getOrder(w http.ResponseWriter, r *http.Request) {
	ctx, endSpan := StartSpan(r.Context(), a.logger, "orders.lookup")
	defer endSpan()
	id, err := strconv.ParseInt(r.PathValue("id"), 10, 64)
	if err != nil || id <= 0 { writeJSON(w, http.StatusBadRequest, map[string]string{"error":"order id must be a positive integer"}); return }
	a.logger.InfoContext(ctx, "order_lookup_completed", "operation", "orders.lookup")
	writeJSON(w, http.StatusOK, orderResponse{ID:id, Status:"processing"})
}
func writeJSON(w http.ResponseWriter, status int, value any) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(value)
}
func readinessAfterStartup(ready *atomic.Bool) { ready.Store(true) }
