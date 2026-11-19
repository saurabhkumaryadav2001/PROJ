package main

import (
	"bytes"
	"context"
	"errors"
	"io"
	"log"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

func TestWaitForReturnsWhenCanceled(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	started := time.Now()
	err := waitFor(ctx, time.Second)
	if !errors.Is(err, context.Canceled) {
		t.Fatalf("error = %v, want context.Canceled", err)
	}
	if time.Since(started) > 100*time.Millisecond {
		t.Fatal("waitFor did not return promptly after cancellation")
	}
}

func TestWaitForDeadline(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Millisecond)
	defer cancel()
	err := waitFor(ctx, time.Second)
	if !errors.Is(err, context.DeadlineExceeded) {
		t.Fatalf("error = %v, want context.DeadlineExceeded", err)
	}
}

func TestChildCancellationDoesNotCancelParent(t *testing.T) {
	parent := context.Background()
	child, cancelChild := context.WithCancel(parent)
	cancelChild()
	if !errors.Is(child.Err(), context.Canceled) {
		t.Fatalf("child error = %v", child.Err())
	}
	if parent.Err() != nil {
		t.Fatalf("parent was canceled: %v", parent.Err())
	}
}

func TestChildInheritsEarlierParentDeadline(t *testing.T) {
	parent, cancelParent := context.WithTimeout(context.Background(), 30*time.Millisecond)
	defer cancelParent()
	child, cancelChild := context.WithTimeout(parent, time.Second)
	defer cancelChild()

	parentDeadline, _ := parent.Deadline()
	childDeadline, _ := child.Deadline()
	if childDeadline.After(parentDeadline) {
		t.Fatalf("child deadline %v is later than parent deadline %v", childDeadline, parentDeadline)
	}
	if err := waitFor(child, time.Second); !errors.Is(err, context.DeadlineExceeded) {
		t.Fatalf("error = %v, want deadline exceeded", err)
	}
}

func TestRequestIDMiddleware(t *testing.T) {
	handler := RequestID(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		id, ok := RequestIDFromContext(r.Context())
		if !ok || id == "" {
			t.Error("request ID missing from context")
		}
		w.WriteHeader(http.StatusNoContent)
	}))

	recorder := httptest.NewRecorder()
	handler.ServeHTTP(recorder, httptest.NewRequest(http.MethodGet, "/", nil))
	if recorder.Header().Get("X-Request-ID") == "" {
		t.Fatal("X-Request-ID response header missing")
	}
}

func TestTimeoutEndpointReturnsGatewayTimeout(t *testing.T) {
	logger := log.New(io.Discard, "", 0)
	handler := NewAPI(logger).Handler()
	recorder := httptest.NewRecorder()
	handler.ServeHTTP(recorder, httptest.NewRequest(http.MethodGet, "/api/v1/timeout", nil))
	if recorder.Code != http.StatusGatewayTimeout {
		t.Fatalf("status = %d, want %d; body=%s", recorder.Code, http.StatusGatewayTimeout, recorder.Body.String())
	}
}

func TestBatchCompletesWithBoundedWorkers(t *testing.T) {
	logger := log.New(io.Discard, "", 0)
	handler := NewAPI(logger).Handler()
	body := `{"tasks":[{"id":"a","duration_ms":10},{"id":"b","duration_ms":10},{"id":"c","duration_ms":10}]}`
	req := httptest.NewRequest(http.MethodPost, "/api/v1/batch", strings.NewReader(body))
	recorder := httptest.NewRecorder()
	handler.ServeHTTP(recorder, req)
	if recorder.Code != http.StatusOK {
		t.Fatalf("status = %d, body=%s", recorder.Code, recorder.Body.String())
	}
	if !strings.Contains(recorder.Body.String(), `"completed":3`) {
		t.Fatalf("unexpected response: %s", recorder.Body.String())
	}
}

func TestRunTasksStopsOnParentCancellation(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	_, err := RunTasks(ctx, []Task{{ID: "slow", Duration: time.Second}}, 2)
	if !errors.Is(err, context.Canceled) {
		t.Fatalf("error = %v, want context.Canceled", err)
	}
}

func TestBatchRejectsUnknownFields(t *testing.T) {
	logger := log.New(io.Discard, "", 0)
	handler := NewAPI(logger).Handler()
	body := `{"tasks":[{"id":"a","duration_ms":10,"unexpected":true}]}`
	req := httptest.NewRequest(http.MethodPost, "/api/v1/batch", bytes.NewBufferString(body))
	recorder := httptest.NewRecorder()
	handler.ServeHTTP(recorder, req)
	if recorder.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want 400", recorder.Code)
	}
}
