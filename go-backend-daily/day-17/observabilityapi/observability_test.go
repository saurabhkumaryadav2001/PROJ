package main

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"
)

func envMap(values map[string]string) func(string) string { return func(key string) string { return values[key] } }

func TestLoadConfigDefaultsAndOverrides(t *testing.T) {
	defaults, err := loadConfig(envMap(nil))
	if err != nil { t.Fatal(err) }
	if defaults.Port != 8080 || defaults.Version != "dev" || defaults.ShutdownTimeout != 10*time.Second { t.Fatalf("unexpected defaults: %+v", defaults) }
	custom, err := loadConfig(envMap(map[string]string{"PORT":"9090", "APP_VERSION":"1.2.3", "READ_TIMEOUT":"8s", "SHUTDOWN_TIMEOUT":"4s"}))
	if err != nil { t.Fatal(err) }
	if custom.Port != 9090 || custom.Version != "1.2.3" || custom.ReadTimeout != 8*time.Second || custom.ShutdownTimeout != 4*time.Second { t.Fatalf("overrides not applied: %+v", custom) }
}

func TestLoadConfigRejectsInvalidValues(t *testing.T) {
	cases := []struct { name string; values map[string]string }{
		{"not a number", map[string]string{"PORT":"abc"}},
		{"port too high", map[string]string{"PORT":"70000"}},
		{"port zero", map[string]string{"PORT":"0"}},
		{"bad timeout", map[string]string{"READ_TIMEOUT":"later"}},
		{"zero timeout", map[string]string{"WRITE_TIMEOUT":"0s"}},
	}
	for _, tc := range cases { t.Run(tc.name, func(t *testing.T) { if _, err := loadConfig(envMap(tc.values)); err == nil { t.Fatal("expected invalid configuration error") } }) }
}

func TestParseTraceParent(t *testing.T) {
	cases := []struct { name, value string; wantOK bool }{
		{"valid", "00-4bf92f3577b34da6a3ce929d0e0e4736-00f067aa0ba902b7-01", true},
		{"wrong version", "01-4bf92f3577b34da6a3ce929d0e0e4736-00f067aa0ba902b7-01", false},
		{"zero trace id", "00-00000000000000000000000000000000-00f067aa0ba902b7-01", false},
		{"zero span id", "00-4bf92f3577b34da6a3ce929d0e0e4736-0000000000000000-01", false},
		{"bad hex", "00-zzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzz-00f067aa0ba902b7-01", false},
		{"missing part", "00-4bf92f3577b34da6a3ce929d0e0e4736-01", false},
	}
	for _, tc := range cases { t.Run(tc.name, func(t *testing.T) { _, ok := ParseTraceParent(tc.value); if ok != tc.wantOK { t.Fatalf("ok=%v want %v", ok, tc.wantOK) } }) }
}

func TestNewServerTraceContinuesIncomingTrace(t *testing.T) {
	parent := "00-4bf92f3577b34da6a3ce929d0e0e4736-00f067aa0ba902b7-01"
	trace, err := NewServerTrace(parent)
	if err != nil { t.Fatal(err) }
	if trace.TraceID != "4bf92f3577b34da6a3ce929d0e0e4736" { t.Fatalf("trace ID not preserved: %+v", trace) }
	if trace.SpanID == "00f067aa0ba902b7" || len(trace.SpanID) != 16 { t.Fatalf("server span ID should be new: %+v", trace) }
}

func TestInjectTraceParent(t *testing.T) {
	trace := TraceContext{TraceID:"4bf92f3577b34da6a3ce929d0e0e4736", SpanID:"00f067aa0ba902b7", Flags:"01"}
	header := make(http.Header)
	InjectTraceParent(withTrace(context.Background(), trace), header)
	if got := header.Get("traceparent"); got != trace.TraceParent() { t.Fatalf("got %q", got) }
}

func TestMetricsPrometheusOutput(t *testing.T) {
	metrics := NewMetrics()
	metrics.Begin()
	metrics.Record("GET", "/healthz", 200, 12*time.Millisecond)
	metrics.End()
	var output bytes.Buffer
	if err := metrics.WritePrometheus(&output); err != nil { t.Fatal(err) }
	for _, expected := range []string{
		"go_http_requests_total{method=\"GET\",route=\"/healthz\",status=\"200\"} 1",
		"go_http_request_duration_seconds_count{method=\"GET\",route=\"/healthz\"} 1",
		"go_http_requests_in_flight 0",
	} {
		if !strings.Contains(output.String(), expected) { t.Fatalf("metrics missing %q:\n%s", expected, output.String()) }
	}
}

func TestMetricsConcurrentRecords(t *testing.T) {
	metrics := NewMetrics()
	const workers, perWorker = 12, 100
	var wg sync.WaitGroup
	wg.Add(workers)
	for i := 0; i < workers; i++ { go func() { defer wg.Done(); for j := 0; j < perWorker; j++ { metrics.Record("GET", "/healthz", 200, time.Millisecond) } }() }
	wg.Wait()
	var output bytes.Buffer
	if err := metrics.WritePrometheus(&output); err != nil { t.Fatal(err) }
	want := fmt.Sprintf("go_http_requests_total{method=\"GET\",route=\"/healthz\",status=\"200\"} %d", workers*perWorker)
	if !strings.Contains(output.String(), want) { t.Fatalf("missing total %d", workers*perWorker) }
}

func testHandler(logOutput *bytes.Buffer) http.Handler {
	logger := slog.New(slog.NewJSONHandler(logOutput, nil))
	ready := &atomic.Bool{}
	ready.Store(true)
	return NewHandler(logger, NewMetrics(), "test", ready)
}

func TestOrderHandlerPropagatesTraceAndCreatesChildSpan(t *testing.T) {
	var logs bytes.Buffer
	handler := testHandler(&logs)
	req := httptest.NewRequest(http.MethodGet, "/api/v1/orders/42", nil)
	req.Header.Set("traceparent", "00-4bf92f3577b34da6a3ce929d0e0e4736-00f067aa0ba902b7-01")
	recorder := httptest.NewRecorder()
	handler.ServeHTTP(recorder, req)
	if recorder.Code != http.StatusOK { t.Fatalf("status=%d body=%s", recorder.Code, recorder.Body.String()) }
	if recorder.Header().Get("X-Trace-ID") != "4bf92f3577b34da6a3ce929d0e0e4736" { t.Fatal("incoming trace ID was not preserved") }
	var body orderResponse
	if err := json.Unmarshal(recorder.Body.Bytes(), &body); err != nil { t.Fatal(err) }
	if body.ID != 42 || body.Status != "processing" { t.Fatalf("unexpected body: %+v", body) }
	if !strings.Contains(logs.String(), "span_finished") || !strings.Contains(logs.String(), "http_request") { t.Fatalf("expected span and request logs, got %s", logs.String()) }
}


func TestRuntimeEndpointReturnsVersionWithoutSecrets(t *testing.T) {
	handler := testHandler(&bytes.Buffer{})
	recorder := httptest.NewRecorder()
	handler.ServeHTTP(recorder, httptest.NewRequest(http.MethodGet, "/api/v1/runtime", nil))
	if recorder.Code != http.StatusOK { t.Fatalf("status=%d", recorder.Code) }
	var body runtimeResponse
	if err := json.Unmarshal(recorder.Body.Bytes(), &body); err != nil { t.Fatal(err) }
	if body.Version != "test" || body.GoVersion == "" { t.Fatalf("unexpected runtime metadata: %+v", body) }
	if strings.Contains(recorder.Body.String(), "DATABASE_URL") || strings.Contains(recorder.Body.String(), "password") { t.Fatal("runtime endpoint exposed a secret") }
}

func TestOrderHandlerRejectsInvalidID(t *testing.T) {
	handler := testHandler(&bytes.Buffer{})
	recorder := httptest.NewRecorder()
	handler.ServeHTTP(recorder, httptest.NewRequest(http.MethodGet, "/api/v1/orders/not-a-number", nil))
	if recorder.Code != http.StatusBadRequest { t.Fatalf("status=%d want 400", recorder.Code) }
}

func TestMetricsEndpointExposesPrometheusText(t *testing.T) {
	handler := testHandler(&bytes.Buffer{})
	first := httptest.NewRecorder()
	handler.ServeHTTP(first, httptest.NewRequest(http.MethodGet, "/healthz", nil))
	metrics := httptest.NewRecorder()
	handler.ServeHTTP(metrics, httptest.NewRequest(http.MethodGet, "/metrics", nil))
	if metrics.Code != http.StatusOK { t.Fatalf("status=%d", metrics.Code) }
	if !strings.Contains(metrics.Body.String(), "go_http_requests_total{method=\"GET\",route=\"/healthz\",status=\"200\"} 1") { t.Fatalf("metrics missing health request: %s", metrics.Body.String()) }
}

func FuzzParseTraceParent(f *testing.F) {
	f.Add("00-4bf92f3577b34da6a3ce929d0e0e4736-00f067aa0ba902b7-01")
	f.Add("not-a-traceparent")
	f.Add("")
	f.Fuzz(func(t *testing.T, input string) {
		trace, ok := ParseTraceParent(input)
		if ok && (len(trace.TraceID) != 32 || len(trace.SpanID) != 16 || len(trace.Flags) != 2) { t.Fatalf("parser accepted invalid lengths: %+v", trace) }
	})
}

func BenchmarkHealthEndpointWithObservability(b *testing.B) {
	logger := slog.New(slog.NewJSONHandler(io.Discard, nil))
	ready := &atomic.Bool{}
	ready.Store(true)
	handler := NewHandler(logger, NewMetrics(), "bench", ready)
	request := httptest.NewRequest(http.MethodGet, "/healthz", nil)
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ { recorder := httptest.NewRecorder(); handler.ServeHTTP(recorder, request) }
}
