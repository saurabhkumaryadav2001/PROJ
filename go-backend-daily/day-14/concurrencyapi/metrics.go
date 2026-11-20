package main

import "sync"

// Metrics protects counters shared by concurrent HTTP handlers.
type Metrics struct {
	mu             sync.RWMutex
	requests       uint64
	active         int64
	completedItems uint64
	failedRequests uint64
}

type MetricsSnapshot struct {
	Requests       uint64 `json:"requests"`
	Active         int64  `json:"active"`
	CompletedItems uint64 `json:"completed_items"`
	FailedRequests uint64 `json:"failed_requests"`
}

func (m *Metrics) BeginRequest() {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.requests++
	m.active++
}

func (m *Metrics) EndRequest(completed int, failed bool) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.active--
	if completed > 0 {
		m.completedItems += uint64(completed)
	}
	if failed {
		m.failedRequests++
	}
}

func (m *Metrics) Snapshot() MetricsSnapshot {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return MetricsSnapshot{
		Requests:       m.requests,
		Active:         m.active,
		CompletedItems: m.completedItems,
		FailedRequests: m.failedRequests,
	}
}
