package middleware

import (
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestCORS(t *testing.T) {
	tests := []struct {
		name            string
		method          string
		origin          string
		wantStatus      int
		wantNext        bool
		wantAllowOrigin bool
	}{
		{"no origin", http.MethodGet, "", http.StatusNoContent, true, false},
		{"disallowed", http.MethodGet, "https://evil.example", http.StatusNoContent, true, false},
		{"allowed regular", http.MethodGet, "http://localhost:3000", http.StatusNoContent, true, true},
		{"allowed preflight", http.MethodOptions, "http://localhost:3000", http.StatusNoContent, false, true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			nextCalled := false
			next := http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
				nextCalled = true
				w.WriteHeader(http.StatusNoContent)
			})

			handler := CORS([]string{"http://localhost:3000"})(next)
			req := httptest.NewRequest(tt.method, "/", nil)
			if tt.origin != "" {
				req.Header.Set("Origin", tt.origin)
			}
			rec := httptest.NewRecorder()

			handler.ServeHTTP(rec, req)

			if rec.Code != tt.wantStatus {
				t.Fatalf("expected %d, got %d", tt.wantStatus, rec.Code)
			}
			if nextCalled != tt.wantNext {
				t.Fatalf("next called=%v, want %v", nextCalled, tt.wantNext)
			}
			if tt.wantAllowOrigin && rec.Header().Get("Access-Control-Allow-Origin") != tt.origin {
				t.Fatalf("unexpected allow-origin header: %q", rec.Header().Get("Access-Control-Allow-Origin"))
			}
			if tt.method == http.MethodOptions && rec.Header().Get("Access-Control-Allow-Methods") == "" {
				t.Fatal("preflight methods header is missing")
			}
		})
	}
}
