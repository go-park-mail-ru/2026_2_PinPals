package middleware

import (
	"context"
	"net/http"
	"strings"

	"github.com/google/uuid"
)

const RequestIDHeader = "X-Request-ID"

type requestIDContextKey struct{}

// ContextWithRequestID stores the identifier shared by every log entry produced
// while handling one HTTP request (including database and WebSocket activity).
func ContextWithRequestID(ctx context.Context, requestID string) context.Context {
	return context.WithValue(ctx, requestIDContextKey{}, requestID)
}

// RequestIDFromContext returns the current request identifier, if present.
func RequestIDFromContext(ctx context.Context) (string, bool) {
	requestID, ok := ctx.Value(requestIDContextKey{}).(string)
	return requestID, ok && requestID != ""
}

// NewRequestID keeps a valid caller-supplied UUID for cross-service correlation
// and generates a new UUID when the header is missing or malformed.
func NewRequestID(value string) string {
	value = strings.TrimSpace(value)
	if parsed, err := uuid.Parse(value); err == nil {
		return parsed.String()
	}
	return uuid.NewString()
}

// RequestID adds a request ID to the context and response headers.
func RequestID(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requestID := NewRequestID(r.Header.Get(RequestIDHeader))
		w.Header().Set(RequestIDHeader, requestID)
		next.ServeHTTP(w, r.WithContext(ContextWithRequestID(r.Context(), requestID)))
	})
}
