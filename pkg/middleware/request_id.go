package middleware

import (
	"context"
	"strings"

	"github.com/google/uuid"
)

const RequestIDHeader = "X-Request-ID"

type requestIDContextKey struct{}

// ContextWithRequestID stores the identifier for the current HTTP request or
// logical operation (for example, one incoming WebSocket message).
func ContextWithRequestID(ctx context.Context, requestID string) context.Context {
	return context.WithValue(ctx, requestIDContextKey{}, requestID)
}

// RequestIDFromContext returns the current request identifier, if present.
func RequestIDFromContext(ctx context.Context) (string, bool) {
	requestID, ok := ctx.Value(requestIDContextKey{}).(string)
	return requestID, ok && requestID != ""
}

// RequestID keeps a valid caller-supplied UUID for cross-service correlation
// and generates a new UUID when the header is missing or malformed.
func RequestID(value string) string {
	value = strings.TrimSpace(value)
	if parsed, err := uuid.Parse(value); err == nil {
		return parsed.String()
	}
	return uuid.NewString()
}
