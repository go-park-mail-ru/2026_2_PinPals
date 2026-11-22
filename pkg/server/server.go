package server

import (
	"log/slog"
	"net/http"
	"time"

	"2026_2_PinPals/pkg/middleware"
)

func New(addr string, handler http.Handler) *http.Server {
	return &http.Server{
		Addr:              addr,
		Handler:           handler,
		ReadHeaderTimeout: 5 * time.Second,
		ReadTimeout:       10 * time.Second,
		WriteTimeout:      15 * time.Second,
		IdleTimeout:       60 * time.Second,
	}
}

// RequestLogger creates one request-scoped logger and attaches its request ID
// to the context before any handler or database operation is executed.
func RequestLogger(logger *slog.Logger) func(http.Handler) http.Handler {
	if logger == nil {
		logger = slog.Default()
	}
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			requestID := middleware.NewRequestID(r.Header.Get(middleware.RequestIDHeader))
			w.Header().Set(middleware.RequestIDHeader, requestID)
			requestLogger := logger.With("request_id", requestID)
			ctx := middleware.ContextWithRequestID(r.Context(), requestID)
			ctx = middleware.ContextWithLogger(ctx, requestLogger)
			requestLogger.InfoContext(ctx, "http request", "method", r.Method, "path", r.URL.Path)
			next.ServeHTTP(w, r.WithContext(ctx))
		})
	}
}
