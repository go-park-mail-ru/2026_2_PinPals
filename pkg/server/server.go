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

func RequestLogger(logger *slog.Logger) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			requestLogger := logger
			if requestLogger == nil {
				requestLogger = slog.Default()
			}
			requestLogger.InfoContext(r.Context(), "http request", "method", r.Method, "path", r.URL.Path)
			next.ServeHTTP(w, r.WithContext(middleware.ContextWithLogger(r.Context(), requestLogger)))
		})
	}
}
