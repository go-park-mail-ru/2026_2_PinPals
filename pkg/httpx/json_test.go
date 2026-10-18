package httpx

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestDecodeJSON(t *testing.T) {
	tests := []struct {
		name        string
		contentType string
		body        string
		wantStatus  int
		wantErr     bool
	}{
		{"valid", "application/json", `{"name":"Alex"}`, http.StatusOK, false},
		{"valid without content type", "", `{"name":"Alex"}`, http.StatusOK, false},
		{"wrong content type", "text/plain", `{"name":"Alex"}`, http.StatusUnsupportedMediaType, true},
		{"invalid json", "application/json", `{`, http.StatusBadRequest, true},
		{"unknown field", "application/json", `{"unknown":1}`, http.StatusBadRequest, true},
		{"extra json value", "application/json", `{"name":"Alex"}{"name":"Bob"}`, http.StatusBadRequest, true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req := httptest.NewRequest(http.MethodPost, "/", bytes.NewBufferString(tt.body))
			if tt.contentType != "" {
				req.Header.Set("Content-Type", tt.contentType)
			}
			rec := httptest.NewRecorder()

			var dst struct {
				Name string `json:"name"`
			}
			err := DecodeJSON(rec, req, &dst)

			if (err != nil) != tt.wantErr {
				t.Fatalf("wantErr=%v, err=%v", tt.wantErr, err)
			}
			if rec.Code != tt.wantStatus {
				t.Fatalf("expected status %d, got %d", tt.wantStatus, rec.Code)
			}
		})
	}
}

func TestWriteJSONAndWriteError(t *testing.T) {
	t.Run("json", func(t *testing.T) {
		rec := httptest.NewRecorder()
		WriteJSON(rec, http.StatusCreated, map[string]string{"ok": "yes"})

		if rec.Code != http.StatusCreated {
			t.Fatalf("expected 201, got %d", rec.Code)
		}
		if rec.Header().Get("Content-Type") != "application/json" {
			t.Fatalf("unexpected content type %q", rec.Header().Get("Content-Type"))
		}
		var body map[string]string
		if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
			t.Fatal(err)
		}
		if body["ok"] != "yes" {
			t.Fatalf("unexpected body: %#v", body)
		}
	})

	t.Run("error", func(t *testing.T) {
		rec := httptest.NewRecorder()
		WriteError(rec, http.StatusBadRequest, "bad request")
		if rec.Code != http.StatusBadRequest {
			t.Fatalf("expected 400, got %d", rec.Code)
		}
	})
}
