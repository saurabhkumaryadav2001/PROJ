package main

import (
	"context"
	"fmt"
	"log"
	"net/http"
	"sync/atomic"
	"time"
)

// contextKey is an unexported type so packages cannot accidentally collide
// with this package's request metadata key.
type contextKey string

const requestIDKey contextKey = "request-id"

func RequestIDFromContext(ctx context.Context) (string, bool) {
	id, ok := ctx.Value(requestIDKey).(string)
	return id, ok
}

func RequestID(next http.Handler) http.Handler {
	var sequence atomic.Uint64

	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		id := fmt.Sprintf("req-%06d", sequence.Add(1))
		ctx := context.WithValue(r.Context(), requestIDKey, id)
		w.Header().Set("X-Request-ID", id)
		next.ServeHTTP(w, r.WithContext(ctx))
	})
}

func Logging(logger *log.Logger) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			started := time.Now()
			next.ServeHTTP(w, r)
			requestID, _ := RequestIDFromContext(r.Context())
			logger.Printf("request_id=%s method=%s path=%s duration=%s context_error=%v",
				requestID, r.Method, r.URL.Path, time.Since(started), r.Context().Err())
		})
	}
}
