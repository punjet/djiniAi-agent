package trace

import (
	"context"

	"github.com/google/uuid"
)

type contextKey string

const traceIDKey contextKey = "trace_id"

// WithTraceID adds a new or existing trace ID to the context.
func WithTraceID(ctx context.Context, id string) context.Context {
	if id == "" {
		id = uuid.New().String()
	}
	return context.WithValue(ctx, traceIDKey, id)
}

// FromContext extracts the trace ID from the context. Returns empty string if not found.
func FromContext(ctx context.Context) string {
	if val, ok := ctx.Value(traceIDKey).(string); ok {
		return val
	}
	return ""
}
