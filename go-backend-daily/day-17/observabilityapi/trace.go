package main

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"log/slog"
	"net/http"
	"strings"
	"time"
)

type TraceContext struct { TraceID, SpanID, Flags string }
type traceContextKey struct{}

func ParseTraceParent(value string) (TraceContext, bool) {
	parts := strings.Split(value, "-")
	if len(parts) != 4 || parts[0] != "00" || len(parts[1]) != 32 || len(parts[2]) != 16 || len(parts[3]) != 2 { return TraceContext{}, false }
	if !validHex(parts[1]) || !validHex(parts[2]) || !validHex(parts[3]) || allZero(parts[1]) || allZero(parts[2]) { return TraceContext{}, false }
	return TraceContext{TraceID: strings.ToLower(parts[1]), SpanID: strings.ToLower(parts[2]), Flags: strings.ToLower(parts[3])}, true
}

func validHex(value string) bool { _, err := hex.DecodeString(value); return err == nil }
func allZero(value string) bool {
	for _, char := range value { if char != '0' { return false } }
	return true
}
func randomHex(byteCount int) (string, error) {
	buffer := make([]byte, byteCount)
	if _, err := rand.Read(buffer); err != nil { return "", err }
	return hex.EncodeToString(buffer), nil
}

func NewServerTrace(parentHeader string) (TraceContext, error) {
	flags, traceID := "01", ""
	if parent, ok := ParseTraceParent(parentHeader); ok {
		traceID, flags = parent.TraceID, parent.Flags
	} else {
		var err error
		traceID, err = randomHex(16)
		if err != nil { return TraceContext{}, err }
	}
	spanID, err := randomHex(8)
	if err != nil { return TraceContext{}, err }
	return TraceContext{TraceID: traceID, SpanID: spanID, Flags: flags}, nil
}

func TraceFromContext(ctx context.Context) (TraceContext, bool) {
	trace, ok := ctx.Value(traceContextKey{}).(TraceContext)
	return trace, ok
}
func withTrace(ctx context.Context, trace TraceContext) context.Context {
	return context.WithValue(ctx, traceContextKey{}, trace)
}
func (t TraceContext) TraceParent() string { return "00-" + t.TraceID + "-" + t.SpanID + "-" + t.Flags }

func InjectTraceParent(ctx context.Context, header http.Header) {
	if trace, ok := TraceFromContext(ctx); ok { header.Set("traceparent", trace.TraceParent()) }
}

// StartSpan is a teaching tracer: it logs span boundaries but does not export OTLP spans.
func StartSpan(ctx context.Context, logger *slog.Logger, name string) (context.Context, func()) {
	parent, ok := TraceFromContext(ctx)
	if !ok { return ctx, func() {} }
	spanID, err := randomHex(8)
	if err != nil { return ctx, func() {} }
	child := TraceContext{TraceID: parent.TraceID, SpanID: spanID, Flags: parent.Flags}
	childContext := withTrace(ctx, child)
	started := time.Now()
	logger.InfoContext(childContext, "span_started", "trace_id", child.TraceID, "span_id", child.SpanID, "parent_span_id", parent.SpanID, "span_name", name)
	end := func() {
		logger.InfoContext(childContext, "span_finished", "trace_id", child.TraceID, "span_id", child.SpanID, "parent_span_id", parent.SpanID, "span_name", name, "duration_ms", time.Since(started).Milliseconds())
	}
	return childContext, end
}
