package main

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"math"
	"net/http"
	"net/http/httptest"
	"sync"
	"testing"
	"time"
)

func TestCalculatePreservesInputOrder(t *testing.T) {
	service, err := NewQuoteService(3, 1)
	if err != nil {
		t.Fatal(err)
	}
	items := []QuoteItem{
		{SKU: "slow", UnitPricePaise: 100, Quantity: 2, ProcessingMS: 60},
		{SKU: "fast", UnitPricePaise: 250, Quantity: 3, ProcessingMS: 5},
		{SKU: "medium", UnitPricePaise: 500, Quantity: 1, ProcessingMS: 25},
	}

	got, err := service.Calculate(context.Background(), items)
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 3 {
		t.Fatalf("got %d quotes, want 3", len(got))
	}
	if got[0].SKU != "slow" || got[1].SKU != "fast" || got[2].SKU != "medium" {
		t.Fatalf("result order changed: %#v", got)
	}
	if got[0].SubtotalPaise != 200 || got[1].SubtotalPaise != 750 {
		t.Fatalf("incorrect subtotal: %#v", got)
	}
}

func TestCalculateStopsOnDeadline(t *testing.T) {
	service, _ := NewQuoteService(2, 1)
	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Millisecond)
	defer cancel()

	items := []QuoteItem{
		{SKU: "A", UnitPricePaise: 100, Quantity: 1, ProcessingMS: 500},
		{SKU: "B", UnitPricePaise: 100, Quantity: 1, ProcessingMS: 500},
		{SKU: "C", UnitPricePaise: 100, Quantity: 1, ProcessingMS: 500},
	}
	started := time.Now()
	_, err := service.Calculate(ctx, items)
	if !errors.Is(err, context.DeadlineExceeded) {
		t.Fatalf("got %v, want deadline exceeded", err)
	}
	if time.Since(started) > 250*time.Millisecond {
		t.Fatal("Calculate did not stop promptly after deadline")
	}
}

func TestCalculateRejectsOverflow(t *testing.T) {
	service, _ := NewQuoteService(1, 1)
	_, err := service.Calculate(context.Background(), []QuoteItem{{
		SKU: "HUGE", UnitPricePaise: math.MaxInt64, Quantity: 2,
	}})
	if !errors.Is(err, ErrAmountOverflow) {
		t.Fatalf("got %v, want ErrAmountOverflow", err)
	}
}

func TestMetricsAreSafeForConcurrentAccess(t *testing.T) {
	metrics := &Metrics{}
	const count = 100
	var wg sync.WaitGroup
	wg.Add(count)
	for i := 0; i < count; i++ {
		go func() {
			defer wg.Done()
			metrics.BeginRequest()
			metrics.EndRequest(2, false)
			_ = metrics.Snapshot()
		}()
	}
	wg.Wait()

	got := metrics.Snapshot()
	if got.Requests != count || got.CompletedItems != count*2 || got.Active != 0 || got.FailedRequests != 0 {
		t.Fatalf("unexpected metrics: %+v", got)
	}
}

func TestHTTPQuotesEndpoint(t *testing.T) {
	service, _ := NewQuoteService(2, 2)
	api := NewAPI(service, &Metrics{})
	body := bytes.NewBufferString(`{"items":[{"sku":"KB-01","unit_price_paise":149900,"quantity":2}]}`)
	req := httptest.NewRequest(http.MethodPost, "/api/v1/quotes", body)
	req.Header.Set("Content-Type", "application/json")
	response := httptest.NewRecorder()

	api.Handler().ServeHTTP(response, req)
	if response.Code != http.StatusOK {
		t.Fatalf("got status %d, body %s", response.Code, response.Body.String())
	}
	var result quoteResponse
	if err := json.Unmarshal(response.Body.Bytes(), &result); err != nil {
		t.Fatal(err)
	}
	if result.Count != 1 || result.Quotes[0].SubtotalPaise != 299800 {
		t.Fatalf("unexpected response: %+v", result)
	}
}

func TestHTTPRejectsUnknownJSONField(t *testing.T) {
	service, _ := NewQuoteService(1, 1)
	api := NewAPI(service, &Metrics{})
	body := bytes.NewBufferString(`{"items":[],"surprise":true}`)
	req := httptest.NewRequest(http.MethodPost, "/api/v1/quotes", body)
	req.Header.Set("Content-Type", "application/json")
	response := httptest.NewRecorder()
	api.Handler().ServeHTTP(response, req)
	if response.Code != http.StatusBadRequest {
		t.Fatalf("got status %d, want 400", response.Code)
	}
}

func TestHTTPMetricsEndpoint(t *testing.T) {
	service, _ := NewQuoteService(1, 1)
	metrics := &Metrics{}
	api := NewAPI(service, metrics)
	metrics.BeginRequest()
	metrics.EndRequest(3, false)
	response := httptest.NewRecorder()
	api.Handler().ServeHTTP(response, httptest.NewRequest(http.MethodGet, "/api/v1/metrics", nil))
	if response.Code != http.StatusOK {
		t.Fatalf("got status %d, want 200", response.Code)
	}
}
