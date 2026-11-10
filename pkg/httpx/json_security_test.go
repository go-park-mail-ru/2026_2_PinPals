package httpx

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestWriteJSONEscapesHTMLForXSSResistance(t *testing.T) {
	rec := httptest.NewRecorder()
	WriteJSON(rec, http.StatusOK, map[string]string{"value": "<script>alert(1)</script>"})
	body := rec.Body.String()
	if strings.Contains(body, "<script>") {
		t.Fatalf("HTML was not escaped: %s", body)
	}
	if !strings.Contains(body, "\\u003cscript\\u003e") {
		t.Fatalf("expected escaped HTML, got %s", body)
	}
}

func TestDecodeJSONRejectsOversizedBody(t *testing.T) {
	body := strings.Repeat(" ", int(MaxJSONBodySize+1))
	req := httptest.NewRequest(http.MethodPost, "/", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	var dst map[string]any
	_ = DecodeJSON(rec, req, &dst)
	if rec.Code != http.StatusRequestEntityTooLarge {
		t.Fatalf("expected 413, got %d", rec.Code)
	}
}

func TestDecodeJSONRejectsLookalikeMediaType(t *testing.T) {
	req := httptest.NewRequest(http.MethodPost, "/", strings.NewReader(`{"ok":true}`))
	req.Header.Set("Content-Type", "application/jsonp")
	rec := httptest.NewRecorder()
	var dst map[string]any
	if err := DecodeJSON(rec, req, &dst); err == nil {
		t.Fatal("expected unsupported media type error")
	}
	if rec.Code != http.StatusUnsupportedMediaType {
		t.Fatalf("expected 415, got %d", rec.Code)
	}
}

func TestDecodeJSONAllowsJSONWithCharset(t *testing.T) {
	req := httptest.NewRequest(http.MethodPost, "/", strings.NewReader(`{"ok":true}`))
	req.Header.Set("Content-Type", "application/json; charset=utf-8")
	rec := httptest.NewRecorder()
	var dst map[string]any
	if err := DecodeJSON(rec, req, &dst); err != nil {
		t.Fatalf("expected valid JSON content type, got %v", err)
	}
	if rec.Code != http.StatusOK {
		t.Fatalf("unexpected status: %d", rec.Code)
	}
}
