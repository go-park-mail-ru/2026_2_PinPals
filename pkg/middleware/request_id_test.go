package middleware

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/google/uuid"
)

func TestNewRequestID(t *testing.T) {
	provided := "8b2519f1-6f25-4c70-b12d-91e2b0f12f73"
	if got := NewRequestID(provided); got != provided {
		t.Fatalf("NewRequestID(valid) = %q, want %q", got, provided)
	}
	generated := NewRequestID("not-a-uuid")
	if _, err := uuid.Parse(generated); err != nil {
		t.Fatalf("NewRequestID(invalid) did not return a UUID: %q", generated)
	}
}

func TestRequestIDMiddlewareAddsHeaderAndContext(t *testing.T) {
	var contextID string
	next := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		contextID, _ = RequestIDFromContext(r.Context())
		w.WriteHeader(http.StatusNoContent)
	})
	recorder := httptest.NewRecorder()
	RequestID(next).ServeHTTP(recorder, httptest.NewRequest(http.MethodGet, "/", nil))

	headerID := recorder.Header().Get(RequestIDHeader)
	if headerID == "" || headerID != contextID {
		t.Fatalf("response request id %q does not match context request id %q", headerID, contextID)
	}
}
