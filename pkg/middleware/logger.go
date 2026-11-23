package middleware

import (
	"context"
	"log/slog"
	"net/http"
)

type loggerContextKey struct{}
type baseLoggerContextKey struct{}

// ContextWithLogger associates a request-scoped logger with the context.
func ContextWithLogger(ctx context.Context, logger *slog.Logger) context.Context {
	if logger == nil {
		logger = slog.Default()
	}
	return context.WithValue(ctx, loggerContextKey{}, logger)
}

// LoggerFromContext returns the request-scoped logger or slog.Default().
func LoggerFromContext(ctx context.Context) *slog.Logger {
	if logger, ok := ctx.Value(loggerContextKey{}).(*slog.Logger); ok && logger != nil {
		return logger
	}
	return slog.Default()
}

func contextWithBaseLogger(ctx context.Context, logger *slog.Logger) context.Context {
	if logger == nil {
		logger = slog.Default()
	}
	return context.WithValue(ctx, baseLoggerContextKey{}, logger)
}

func baseLoggerFromContext(ctx context.Context) *slog.Logger {
	if logger, ok := ctx.Value(baseLoggerContextKey{}).(*slog.Logger); ok && logger != nil {
		return logger
	}
	return slog.Default()
}

// RequestLogger is the single HTTP middleware responsible for creating the
// request ID, exposing it in the response header, and attaching a scoped logger
// to the request context. Database tracers and handlers use that same context.
func RequestLogger(logger *slog.Logger) func(http.Handler) http.Handler {
	if logger == nil {
		logger = slog.Default()
	}

	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			requestID := RequestID(r.Header.Get(RequestIDHeader))
			w.Header().Set(RequestIDHeader, requestID)

			ctx := contextWithBaseLogger(r.Context(), logger)
			ctx = RequestContext(ctx, requestID)
			requestLogger := LoggerFromContext(ctx)
			requestLogger.InfoContext(ctx, "http request", "method", r.Method, "path", r.URL.Path)

			next.ServeHTTP(w, r.WithContext(ctx))
		})
	}
}

func RequestContext(parent context.Context, requestID string) context.Context {
	if requestID == "" {
		requestID = RequestID("")
	}
	ctx := ContextWithRequestID(parent, requestID)
	return ContextWithLogger(ctx, baseLoggerFromContext(parent).With("request_id", requestID))
}

func OperationContext(parent context.Context) context.Context {
	return RequestContext(parent, RequestID(""))
}
