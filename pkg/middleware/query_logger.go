package middleware

import (
	"context"
	"strings"
	"time"

	"github.com/jackc/pgx/v5"
)

type queryTraceState struct {
	startedAt time.Time
	sql       string
}

type queryStartContextKey struct{}

// QueryLogger implements pgx.QueryTracer. Query arguments are deliberately not
// logged: they may contain passwords, tokens, message bodies, or personal data.
type QueryLogger struct{}

func NewQueryLogger() *QueryLogger { return &QueryLogger{} }

func (t *QueryLogger) TraceQueryStart(ctx context.Context, _ *pgx.Conn, data pgx.TraceQueryStartData) context.Context {
	return context.WithValue(ctx, queryStartContextKey{}, queryTraceState{
		startedAt: time.Now(),
		sql:       strings.TrimSpace(data.SQL),
	})
}

func (t *QueryLogger) TraceQueryEnd(ctx context.Context, _ *pgx.Conn, data pgx.TraceQueryEndData) {
	logger := LoggerFromContext(ctx)
	state, _ := ctx.Value(queryStartContextKey{}).(queryTraceState)
	attributes := []any{
		"query", state.sql,
		"duration", time.Since(state.startedAt),
		"command_tag", data.CommandTag.String(),
	}
	if data.Err != nil {
		attributes = append(attributes, "error", data.Err)
		logger.ErrorContext(ctx, "database query failed", attributes...)
		return
	}
	logger.InfoContext(ctx, "database query completed", attributes...)
}

var _ pgx.QueryTracer = (*QueryLogger)(nil)
