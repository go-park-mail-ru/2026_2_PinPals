package middleware

import (
	"testing"

	"github.com/google/uuid"
)

func TestRequestID(t *testing.T) {
	provided := "8b2519f1-6f25-4c70-b12d-91e2b0f12f73"
	if got := RequestID(provided); got != provided {
		t.Fatalf("RequestID(valid) = %q, want %q", got, provided)
	}
	generated := RequestID("not-a-uuid")
	if _, err := uuid.Parse(generated); err != nil {
		t.Fatalf("RequestID(invalid) did not return a UUID: %q", generated)
	}
}
