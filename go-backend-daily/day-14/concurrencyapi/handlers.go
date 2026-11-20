package main

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"strings"
	"time"
)

type API struct {
	quotes  *QuoteService
	metrics *Metrics
}

type quoteRequest struct {
	Items []QuoteItem `json:"items"`
}

type quoteResponse struct {
	Count  int     `json:"count"`
	Quotes []Quote `json:"quotes"`
}

type errorResponse struct {
	Error string `json:"error"`
}

func NewAPI(quotes *QuoteService, metrics *Metrics) *API {
	return &API{quotes: quotes, metrics: metrics}
}

func (api *API) Handler() http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("GET /healthz", api.health)
	mux.HandleFunc("GET /api/v1/metrics", api.getMetrics)
	mux.HandleFunc("POST /api/v1/quotes", api.createQuotes)
	return mux
}

func (api *API) health(w http.ResponseWriter, _ *http.Request) {
	writeJSON(w, http.StatusOK, map[string]string{"status": "ok"})
}

func (api *API) getMetrics(w http.ResponseWriter, _ *http.Request) {
	writeJSON(w, http.StatusOK, api.metrics.Snapshot())
}

func (api *API) createQuotes(w http.ResponseWriter, r *http.Request) {
	api.metrics.BeginRequest()
	completed := 0
	failed := true
	defer func() { api.metrics.EndRequest(completed, failed) }()

	if !strings.HasPrefix(r.Header.Get("Content-Type"), "application/json") {
		writeError(w, http.StatusUnsupportedMediaType, "Content-Type must be application/json")
		return
	}

	r.Body = http.MaxBytesReader(w, r.Body, 64<<10)
	decoder := json.NewDecoder(r.Body)
	decoder.DisallowUnknownFields()

	var request quoteRequest
	if err := decoder.Decode(&request); err != nil {
		writeError(w, http.StatusBadRequest, "invalid JSON request")
		return
	}
	if err := decoder.Decode(&struct{}{}); !errors.Is(err, io.EOF) {
		writeError(w, http.StatusBadRequest, "request must contain exactly one JSON object")
		return
	}
	if len(request.Items) == 0 || len(request.Items) > 100 {
		writeError(w, http.StatusUnprocessableEntity, "items must contain between 1 and 100 entries")
		return
	}
	for _, item := range request.Items {
		if strings.TrimSpace(item.SKU) == "" || item.UnitPricePaise <= 0 || item.Quantity <= 0 || item.ProcessingMS < 0 || item.ProcessingMS > 2000 {
			writeError(w, http.StatusUnprocessableEntity, "each item needs a SKU, positive price and quantity, and processing_ms between 0 and 2000")
			return
		}
	}

	// Respect the caller's cancellation while placing a five-second upper bound
	// on the complete batch. An earlier client deadline still wins.
	ctx, cancel := context.WithTimeout(r.Context(), 5*time.Second)
	defer cancel()

	quotes, err := api.quotes.Calculate(ctx, request.Items)
	if err != nil {
		if errors.Is(err, context.DeadlineExceeded) {
			writeError(w, http.StatusGatewayTimeout, "quote calculation exceeded its deadline")
			return
		}
		if errors.Is(err, context.Canceled) {
			writeError(w, http.StatusRequestTimeout, "quote calculation was canceled")
			return
		}
		if errors.Is(err, ErrAmountOverflow) {
			writeError(w, http.StatusUnprocessableEntity, "quote amount exceeds supported range")
			return
		}
		if errors.Is(err, ErrInvalidItem) {
			writeError(w, http.StatusUnprocessableEntity, "invalid quote item")
			return
		}
		writeError(w, http.StatusInternalServerError, "quote calculation failed")
		return
	}

	completed = len(quotes)
	failed = false
	writeJSON(w, http.StatusOK, quoteResponse{Count: len(quotes), Quotes: quotes})
}

func writeJSON(w http.ResponseWriter, status int, value any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(value)
}

func writeError(w http.ResponseWriter, status int, message string) {
	writeJSON(w, status, errorResponse{Error: message})
}
