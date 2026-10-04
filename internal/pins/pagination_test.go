package pins

import (
	"testing"
	"time"
)

func TestCursorRoundTrip(t *testing.T) {
	original := Cursor{
		CreatedAt: time.Date(2026, 10, 4, 1, 2, 3, 0, time.UTC),
		ID:        42,
	}

	encoded, err := encodeCursor(original)
	if err != nil {
		t.Fatal(err)
	}

	decoded, err := decodeCursor(encoded)
	if err != nil {
		t.Fatal(err)
	}

	if decoded.ID != original.ID || !decoded.CreatedAt.Equal(original.CreatedAt) {
		t.Fatalf("unexpected cursor: %#v", decoded)
	}
}

func TestDecodeCursorErrors(t *testing.T) {
	tests := []string{
		"not-base64",
		"e30", // {}
	}

	for _, value := range tests {
		if _, err := decodeCursor(value); err == nil {
			t.Fatalf("expected error for %q", value)
		}
	}
}

func TestDecodeCursorInvalidValues(t *testing.T) {
	zeroID, _ := encodeCursor(Cursor{
		CreatedAt: time.Now(),
		ID:        0,
	})
	if _, err := decodeCursor(zeroID); err == nil {
		t.Fatal("expected invalid id error")
	}

	zeroTime, _ := encodeCursor(Cursor{ID: 1})
	if _, err := decodeCursor(zeroTime); err == nil {
		t.Fatal("expected invalid time error")
	}
}
