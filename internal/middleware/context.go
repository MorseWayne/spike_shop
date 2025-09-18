package middleware

import (
	"context"
)

type contextKey string

const (
	contextKeyRequestID contextKey = "request_id"
)

func withRequestID(ctx context.Context, id string) context.Context {
	return context.WithValue(ctx, contextKeyRequestID, id)
}

// RequestIDFromContext returns request id if present.
func RequestIDFromContext(ctx context.Context) string {
	if v := ctx.Value(contextKeyRequestID); v != nil {
		if s, ok := v.(string); ok {
			return s
		}
	}
	return ""
}
