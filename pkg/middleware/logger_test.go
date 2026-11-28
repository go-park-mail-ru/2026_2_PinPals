package middleware

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestRequestLoggerAddsRequestIDToContextHeaderAndLogs(t *testing.T) {
	var output bytes.Buffer
	logger := slog.New(slog.NewJSONHandler(&output, nil))
	const suppliedID = "8b2519f1-6f25-4c70-b12d-91e2b0f12f73"

	next := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		contextID, ok := RequestIDFromContext(r.Context())
		if !ok || contextID != suppliedID {
			t.Errorf("request context ID = %q, want %q", contextID, suppliedID)
		}
		LoggerFromContext(r.Context()).InfoContext(r.Context(), "handler log")
		w.WriteHeader(http.StatusNoContent)
	})

	req := httptest.NewRequest(http.MethodGet, "/test", nil)
	req.Header.Set(RequestIDHeader, suppliedID)
	recorder := httptest.NewRecorder()
	RequestLogger(logger)(next).ServeHTTP(recorder, req)

	if recorder.Code != http.StatusNoContent {
		t.Fatalf("status = %d, want %d", recorder.Code, http.StatusNoContent)
	}
	if got := recorder.Header().Get(RequestIDHeader); got != suppliedID {
		t.Fatalf("response request ID = %q, want %q", got, suppliedID)
	}

	assertLogRequestIDs(t, &output, []string{suppliedID, suppliedID})
}

func TestOperationContextGetsIndependentRequestIDAndLogger(t *testing.T) {
	var output bytes.Buffer
	baseLogger := slog.New(slog.NewJSONHandler(&output, nil))
	parent := contextWithBaseLogger(context.Background(), baseLogger)
	connectionCtx := RequestContext(parent, "8b2519f1-6f25-4c70-b12d-91e2b0f12f73")

	first := OperationContext(connectionCtx)
	second := OperationContext(connectionCtx)
	firstID, firstOK := RequestIDFromContext(first)
	secondID, secondOK := RequestIDFromContext(second)
	if !firstOK || !secondOK {
		t.Fatal("operation context is missing request ID")
	}
	if firstID == secondID || firstID == "8b2519f1-6f25-4c70-b12d-91e2b0f12f73" || secondID == "8b2519f1-6f25-4c70-b12d-91e2b0f12f73" {
		t.Fatalf("operation IDs must be unique and differ from handshake ID: first=%q second=%q", firstID, secondID)
	}

	LoggerFromContext(first).InfoContext(first, "first websocket operation")
	LoggerFromContext(second).InfoContext(second, "second websocket operation")
	assertLogRequestIDs(t, &output, []string{firstID, secondID})
}

func assertLogRequestIDs(t *testing.T, output *bytes.Buffer, expectedIDs []string) {
	t.Helper()
	scanner := bufio.NewScanner(output)
	lines := 0
	for scanner.Scan() {
		var entry map[string]any
		if err := json.Unmarshal(scanner.Bytes(), &entry); err != nil {
			t.Fatalf("invalid JSON log: %v", err)
		}
		requestID, ok := entry["request_id"].(string)
		if !ok || requestID == "" {
			t.Fatalf("log entry has no request_id: %v", entry)
		}
		if lines >= len(expectedIDs) {
			t.Fatalf("got more log entries than expected: %d", len(expectedIDs))
		}
		if requestID != expectedIDs[lines] {
			t.Fatalf("log request_id = %q, want %q", requestID, expectedIDs[lines])
		}
		lines++
	}
	if err := scanner.Err(); err != nil {
		t.Fatalf("scan logs: %v", err)
	}
	if lines != len(expectedIDs) {
		t.Fatalf("got %d log entries, want %d", lines, len(expectedIDs))
	}
}
