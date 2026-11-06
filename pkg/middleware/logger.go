package middleware

import (
	"context"
	"log/slog"
	"net/http"
)

type loggerContextKey struct{}

// ContextWithLogger связывает логгер с контекстом запроса.
func ContextWithLogger(ctx context.Context, logger *slog.Logger) context.Context {
	if logger == nil {
		logger = slog.Default()
	}
	return context.WithValue(ctx, loggerContextKey{}, logger)
}

// LoggerFromContext возвращает логгер запроса или slog.Default().
func LoggerFromContext(ctx context.Context) *slog.Logger {
	if logger, ok := ctx.Value(loggerContextKey{}).(*slog.Logger); ok && logger != nil {
		return logger
	}
	return slog.Default()
}

// Logger прокидывает логгер через request context.
func Logger(logger *slog.Logger) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			next.ServeHTTP(w, r.WithContext(ContextWithLogger(r.Context(), logger)))
		})
	}
}
