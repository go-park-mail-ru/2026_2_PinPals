package server

import (
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

func TestNewSetsTimeouts(t *testing.T) {
	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {})
	srv := New(":8080", handler)

	if srv.Addr != ":8080" {
		t.Fatalf("unexpected addr: %s", srv.Addr)
	}
	if srv.ReadHeaderTimeout != 5*time.Second {
		t.Fatalf("unexpected read header timeout: %v", srv.ReadHeaderTimeout)
	}
	if srv.ReadTimeout != 10*time.Second {
		t.Fatalf("unexpected read timeout: %v", srv.ReadTimeout)
	}
	if srv.WriteTimeout != 15*time.Second {
		t.Fatalf("unexpected write timeout: %v", srv.WriteTimeout)
	}
	if srv.IdleTimeout != 60*time.Second {
		t.Fatalf("unexpected idle timeout: %v", srv.IdleTimeout)
	}
}

func TestRequestLogger(t *testing.T) {
	var called bool
	next := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		called = true
		w.WriteHeader(http.StatusNoContent)
	})

	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	handler := RequestLogger(logger)(next)

	req := httptest.NewRequest(http.MethodGet, "/test", nil)
	rec := httptest.NewRecorder()

	handler.ServeHTTP(rec, req)

	if !called {
		t.Fatal("next handler was not called")
	}
	if rec.Code != http.StatusNoContent {
		t.Fatalf("expected 204, got %d", rec.Code)
	}
}
