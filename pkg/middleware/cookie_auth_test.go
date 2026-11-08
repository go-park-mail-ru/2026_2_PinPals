package middleware

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

type memoryRevocations map[string]bool

func (m memoryRevocations) Revoke(_ context.Context, id string, _ time.Time) error {
	m[id] = true
	return nil
}
func (m memoryRevocations) IsRevoked(_ context.Context, id string) (bool, error) { return m[id], nil }

func TestMiddlewareReadsCookieAndRejectsRevokedToken(t *testing.T) {
	manager := NewTokenManager([]byte("12345678901234567890123456789012"), 3600)
	store := memoryRevocations{}
	manager.SetRevocationStore(store)
	token, err := manager.Issue(7)
	if err != nil {
		t.Fatal(err)
	}
	req := httptest.NewRequest(http.MethodGet, "/private", nil)
	req.AddCookie(&http.Cookie{Name: AccessTokenCookieName, Value: token})
	rec := httptest.NewRecorder()
	manager.Middleware(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if id, ok := UserIDFromContext(r.Context()); !ok || id != 7 {
			t.Errorf("unexpected user id: %d, %v", id, ok)
		}
		w.WriteHeader(http.StatusNoContent)
	})).ServeHTTP(rec, req)
	if rec.Code != http.StatusNoContent {
		t.Fatalf("expected 204, got %d", rec.Code)
	}
	if err := manager.Revoke(context.Background(), token); err != nil {
		t.Fatal(err)
	}
	rec = httptest.NewRecorder()
	manager.Middleware(http.HandlerFunc(func(http.ResponseWriter, *http.Request) {})).ServeHTTP(rec, req)
	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("expected revoked token to be rejected, got %d", rec.Code)
	}
}

func TestCSRFMiddleware(t *testing.T) {
	h := CSRF(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) { w.WriteHeader(http.StatusNoContent) }))
	req := httptest.NewRequest(http.MethodPost, "/mutate", nil)
	req.AddCookie(&http.Cookie{Name: AccessTokenCookieName, Value: "jwt"})
	req.AddCookie(&http.Cookie{Name: CSRFTokenCookieName, Value: "csrf-value"})
	bad := httptest.NewRecorder()
	h.ServeHTTP(bad, req)
	if bad.Code != http.StatusForbidden {
		t.Fatalf("expected 403, got %d", bad.Code)
	}
	req.Header.Set(CSRFHeaderName, "csrf-value")
	good := httptest.NewRecorder()
	h.ServeHTTP(good, req)
	if good.Code != http.StatusNoContent {
		t.Fatalf("expected 204, got %d", good.Code)
	}
}

func TestSetAuthCookiesSecureFlags(t *testing.T) {
	rec := httptest.NewRecorder()
	SetAuthCookies(rec, "jwt", "csrf", time.Hour, true)
	cookies := rec.Result().Cookies()
	if len(cookies) != 2 {
		t.Fatalf("expected two cookies, got %d", len(cookies))
	}
	for _, cookie := range cookies {
		if !cookie.Secure || cookie.SameSite != http.SameSiteLaxMode || cookie.Path != "/" {
			t.Fatalf("unexpected cookie settings: %#v", cookie)
		}
		if cookie.Name == AccessTokenCookieName && !cookie.HttpOnly {
			t.Fatal("access cookie must be HttpOnly")
		}
		if cookie.Name == CSRFTokenCookieName && cookie.HttpOnly {
			t.Fatal("CSRF cookie must be readable by the frontend")
		}
	}
}
