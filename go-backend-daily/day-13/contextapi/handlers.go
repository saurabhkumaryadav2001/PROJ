package main

import (
	"context"
	"encoding/json"
	"errors"
	"log"
	"net/http"
	"time"
)

type API struct {
	logger *log.Logger
}

func NewAPI(logger *log.Logger) *API {
	return &API{logger: logger}
}

func (api *API) Handler() http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("GET /healthz", api.health)
	mux.HandleFunc("GET /api/v1/request-info", api.requestInfo)
	mux.HandleFunc("GET /api/v1/work", api.work)
	mux.HandleFunc("GET /api/v1/timeout", api.timeoutDemo)
	mux.HandleFunc("POST /api/v1/batch", api.batch)

	// RequestID is outermost, so Logging can read the ID from the request context.
	return RequestID(Logging(api.logger)(mux))
}

func (api *API) health(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, http.StatusOK, map[string]string{"status": "ok"})
}

func (api *API) requestInfo(w http.ResponseWriter, r *http.Request) {
	id, _ := RequestIDFromContext(r.Context())
	deadline, hasDeadline := r.Context().Deadline()
	writeJSON(w, http.StatusOK, map[string]any{
		"request_id":   id,
		"has_deadline": hasDeadline,
		"deadline":     deadline,
		"context_err":  r.Context().Err(),
	})
}

func (api *API) work(w http.ResponseWriter, r *http.Request) {
	duration := 500 * time.Millisecond
	if raw := r.URL.Query().Get("duration"); raw != "" {
		parsed, err := time.ParseDuration(raw)
		if err != nil || parsed <= 0 || parsed > 5*time.Second {
			writeError(w, http.StatusBadRequest, "duration must be between 1ns and 5s, for example 250ms")
			return
		}
		duration = parsed
	}

	if err := waitFor(r.Context(), duration); err != nil {
		api.writeContextError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"status": "completed", "duration": duration.String()})
}

func (api *API) timeoutDemo(w http.ResponseWriter, r *http.Request) {
	// The child deadline is 250ms. The simulated operation needs 2s.
	ctx, cancel := context.WithTimeout(r.Context(), 250*time.Millisecond)
	defer cancel()

	if err := waitFor(ctx, 2*time.Second); err != nil {
		if errors.Is(err, context.DeadlineExceeded) {
			writeError(w, http.StatusGatewayTimeout, "operation exceeded its 250ms deadline")
			return
		}
		api.writeContextError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]string{"status": "completed"})
}

type batchRequest struct {
	Tasks []struct {
		ID         string `json:"id"`
		DurationMS int    `json:"duration_ms"`
	} `json:"tasks"`
}

func (api *API) batch(w http.ResponseWriter, r *http.Request) {
	r.Body = http.MaxBytesReader(w, r.Body, 64<<10)
	decoder := json.NewDecoder(r.Body)
	decoder.DisallowUnknownFields()
	var input batchRequest
	if err := decoder.Decode(&input); err != nil {
		writeError(w, http.StatusBadRequest, "invalid JSON body")
		return
	}
	if len(input.Tasks) == 0 || len(input.Tasks) > 20 {
		writeError(w, http.StatusUnprocessableEntity, "tasks must contain between 1 and 20 items")
		return
	}

	tasks := make([]Task, 0, len(input.Tasks))
	for _, item := range input.Tasks {
		if item.ID == "" || item.DurationMS < 0 || item.DurationMS > 5000 {
			writeError(w, http.StatusUnprocessableEntity, "each task needs an ID and duration_ms between 0 and 5000")
			return
		}
		tasks = append(tasks, Task{ID: item.ID, Duration: time.Duration(item.DurationMS) * time.Millisecond})
	}

	results, err := RunTasks(r.Context(), tasks, 3)
	if err != nil {
		api.writeContextError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"completed": len(results), "results": results})
}

func (api *API) writeContextError(w http.ResponseWriter, err error) {
	switch {
	case errors.Is(err, context.DeadlineExceeded):
		writeError(w, http.StatusGatewayTimeout, "operation deadline exceeded")
	case errors.Is(err, context.Canceled):
		// If the client disconnected, it may no longer receive this response.
		writeError(w, http.StatusRequestTimeout, "request was canceled")
	default:
		api.logger.Printf("unexpected operation error: %v", err)
		writeError(w, http.StatusInternalServerError, "operation failed")
	}
}

func writeJSON(w http.ResponseWriter, status int, value any) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(value)
}

func writeError(w http.ResponseWriter, status int, message string) {
	writeJSON(w, status, map[string]string{"error": message})
}
