package main

import (
	"fmt"
	"io"
	"sort"
	"sync"
	"time"
)

var latencyBounds = [...]float64{0.005, 0.01, 0.025, 0.05, 0.1, 0.25, 0.5, 1, 2.5}

type requestKey struct { method, route string; status int }
type histogramKey struct { method, route string }
type latencyHistogram struct {
	buckets [len(latencyBounds)]uint64
	sum float64
	count uint64
}

type Metrics struct {
	mu sync.Mutex
	requests map[requestKey]uint64
	latency map[histogramKey]*latencyHistogram
	inFlight int64
}

func NewMetrics() *Metrics {
	return &Metrics{requests: make(map[requestKey]uint64), latency: make(map[histogramKey]*latencyHistogram)}
}

func (m *Metrics) Begin() { m.mu.Lock(); m.inFlight++; m.mu.Unlock() }
func (m *Metrics) End() {
	m.mu.Lock()
	if m.inFlight > 0 { m.inFlight-- }
	m.mu.Unlock()
}

func (m *Metrics) Record(method, route string, status int, elapsed time.Duration) {
	seconds := elapsed.Seconds()
	key := requestKey{method: method, route: route, status: status}
	hKey := histogramKey{method: method, route: route}
	m.mu.Lock()
	defer m.mu.Unlock()
	m.requests[key]++
	h := m.latency[hKey]
	if h == nil { h = &latencyHistogram{}; m.latency[hKey] = h }
	h.count++
	h.sum += seconds
	for i, bound := range latencyBounds { if seconds <= bound { h.buckets[i]++ } }
}

func (m *Metrics) WritePrometheus(w io.Writer) error {
	// Snapshot under lock, then write outside the lock so a slow scraper cannot block request recording.
	m.mu.Lock()
	requests := make(map[requestKey]uint64, len(m.requests))
	for key, value := range m.requests { requests[key] = value }
	histograms := make(map[histogramKey]latencyHistogram, len(m.latency))
	for key, value := range m.latency { histograms[key] = *value }
	inFlight := m.inFlight
	m.mu.Unlock()

	if _, err := fmt.Fprintln(w, "# HELP go_http_requests_total Total HTTP requests."); err != nil { return err }
	if _, err := fmt.Fprintln(w, "# TYPE go_http_requests_total counter"); err != nil { return err }
	requestKeys := make([]requestKey, 0, len(requests))
	for key := range requests { requestKeys = append(requestKeys, key) }
	sort.Slice(requestKeys, func(i, j int) bool {
		a, b := requestKeys[i], requestKeys[j]
		if a.method != b.method { return a.method < b.method }
		if a.route != b.route { return a.route < b.route }
		return a.status < b.status
	})
	for _, key := range requestKeys {
		if _, err := fmt.Fprintf(w, "go_http_requests_total{method=%q,route=%q,status=%q} %d\n", key.method, key.route, fmt.Sprint(key.status), requests[key]); err != nil { return err }
	}

	if _, err := fmt.Fprintln(w, "# HELP go_http_request_duration_seconds HTTP request duration in seconds."); err != nil { return err }
	if _, err := fmt.Fprintln(w, "# TYPE go_http_request_duration_seconds histogram"); err != nil { return err }
	histogramKeys := make([]histogramKey, 0, len(histograms))
	for key := range histograms { histogramKeys = append(histogramKeys, key) }
	sort.Slice(histogramKeys, func(i, j int) bool {
		a, b := histogramKeys[i], histogramKeys[j]
		if a.method != b.method { return a.method < b.method }
		return a.route < b.route
	})
	for _, key := range histogramKeys {
		h := histograms[key]
		for i, bound := range latencyBounds {
			if _, err := fmt.Fprintf(w, "go_http_request_duration_seconds_bucket{method=%q,route=%q,le=%q} %d\n", key.method, key.route, fmt.Sprint(bound), h.buckets[i]); err != nil { return err }
		}
		if _, err := fmt.Fprintf(w, "go_http_request_duration_seconds_bucket{method=%q,route=%q,le=\"+Inf\"} %d\n", key.method, key.route, h.count); err != nil { return err }
		if _, err := fmt.Fprintf(w, "go_http_request_duration_seconds_sum{method=%q,route=%q} %g\n", key.method, key.route, h.sum); err != nil { return err }
		if _, err := fmt.Fprintf(w, "go_http_request_duration_seconds_count{method=%q,route=%q} %d\n", key.method, key.route, h.count); err != nil { return err }
	}
	if _, err := fmt.Fprintln(w, "# HELP go_http_requests_in_flight Current number of requests being handled."); err != nil { return err }
	if _, err := fmt.Fprintln(w, "# TYPE go_http_requests_in_flight gauge"); err != nil { return err }
	_, err := fmt.Fprintf(w, "go_http_requests_in_flight %d\n", inFlight)
	return err
}
