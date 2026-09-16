package trace

import (
	"context"

	"github.com/google/uuid"
)

type contextKey string

const traceIDKey contextKey = "trace_id"

func WithTraceID(ctx context.Context, id string) context.Context {
	if id == "" {
		id = uuid.New().String()
	}
	return context.WithValue(ctx, traceIDKey, id)
}
