package main

import (
	"log/slog"
	"net/http"
	"strings"
	"time"
)

func ObservabilityMiddleware(metrics *Metrics, logger *slog.Logger, next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		trace, err := NewServerTrace(r.Header.Get("traceparent"))
		if err != nil { http.Error(w, "unable to initialize trace", http.StatusInternalServerError); return }
		r = r.WithContext(withTrace(r.Context(), trace))
		w.Header().Set("X-Trace-ID", trace.TraceID)

		started := time.Now()
		metrics.Begin()
		recorder := &statusRecorder{ResponseWriter: w, status: http.StatusOK}
		defer func() {
			metrics.End()
			route := strings.TrimPrefix(r.Pattern, r.Method+" ")
			if route == "" { route = "unmatched" }
			elapsed := time.Since(started)
			metrics.Record(r.Method, route, recorder.status, elapsed)
			logger.InfoContext(r.Context(), "http_request", "trace_id", trace.TraceID, "span_id", trace.SpanID, "method", r.Method, "route", route, "status", recorder.status, "duration_ms", elapsed.Milliseconds())
		}()
		next.ServeHTTP(recorder, r)
	})
}

type statusRecorder struct {
	http.ResponseWriter
	status int
	wroteHeader bool
}
func (w *statusRecorder) WriteHeader(status int) {
	if w.wroteHeader { return }
	w.status, w.wroteHeader = status, true
	w.ResponseWriter.WriteHeader(status)
}
func (w *statusRecorder) Write(data []byte) (int, error) {
	if !w.wroteHeader { w.WriteHeader(http.StatusOK) }
	return w.ResponseWriter.Write(data)
}
func (w *statusRecorder) Unwrap() http.ResponseWriter { return w.ResponseWriter }
