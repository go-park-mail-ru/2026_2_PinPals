package middleware

import (
	"bytes"
	"context"
	"log/slog"
	"strings"
	"testing"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
)

func TestQueryLoggerCorrelatesQueriesWithoutLoggingArguments(t *testing.T) {
	var output bytes.Buffer
	logger := slog.New(slog.NewJSONHandler(&output, nil)).With("request_id", "8b2519f1-6f25-4c70-b12d-91e2b0f12f73")
	ctx := ContextWithLogger(context.Background(), logger)

	tracer := NewQueryLogger()
	ctx = tracer.TraceQueryStart(ctx, nil, pgx.TraceQueryStartData{
		SQL:  " SELECT * FROM users WHERE password = $1 ",
		Args: []any{"sensitive-password"},
	})
	tracer.TraceQueryEnd(ctx, nil, pgx.TraceQueryEndData{
		CommandTag: pgconn.NewCommandTag("SELECT 1"),
	})

	logLine := output.String()
	if !strings.Contains(logLine, "8b2519f1-6f25-4c70-b12d-91e2b0f12f73") {
		t.Fatalf("query log does not contain request id: %s", logLine)
	}
	if !strings.Contains(logLine, "SELECT * FROM users WHERE password = $1") {
		t.Fatalf("query log does not contain SQL: %s", logLine)
	}
	if strings.Contains(logLine, "sensitive-password") {
		t.Fatalf("query log contains SQL argument: %s", logLine)
	}
}

func TestQueryLoggerUsesIndependentWebSocketOperationRequestID(t *testing.T) {
	var output bytes.Buffer
	baseLogger := slog.New(slog.NewJSONHandler(&output, nil))
	connectionCtx := RequestContext(contextWithBaseLogger(context.Background(), baseLogger), "8b2519f1-6f25-4c70-b12d-91e2b0f12f73")
	messageCtx := OperationContext(connectionCtx)
	requestID, ok := RequestIDFromContext(messageCtx)
	if !ok || requestID == "8b2519f1-6f25-4c70-b12d-91e2b0f12f73" {
		t.Fatalf("WebSocket operation should have its own request ID, got %q", requestID)
	}

	tracer := NewQueryLogger()
	messageCtx = tracer.TraceQueryStart(messageCtx, nil, pgx.TraceQueryStartData{SQL: "SELECT 1"})
	tracer.TraceQueryEnd(messageCtx, nil, pgx.TraceQueryEndData{CommandTag: pgconn.NewCommandTag("SELECT 1")})

	if !strings.Contains(output.String(), requestID) {
		t.Fatalf("database query log does not contain WebSocket operation request ID %q: %s", requestID, output.String())
	}
	if strings.Contains(output.String(), "8b2519f1-6f25-4c70-b12d-91e2b0f12f73") {
		t.Fatalf("database query log incorrectly uses WebSocket handshake request ID: %s", output.String())
	}
}
