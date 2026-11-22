package main

import (
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestCheckHealth(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))
	t.Cleanup(server.Close)

	if err := checkHealth(server.URL); err != nil {
		t.Fatalf("checkHealth returned error: %v", err)
	}
}

func TestCheckHealthRejectsUnhealthyStatus(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusServiceUnavailable)
	}))
	t.Cleanup(server.Close)

	if err := checkHealth(server.URL); err == nil {
		t.Fatal("expected health check to reject HTTP 503")
	}
}
