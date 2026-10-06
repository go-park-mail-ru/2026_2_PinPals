package middleware

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/golang-jwt/jwt/v5"
)

func TestTokenManagerIssueAndMiddleware(t *testing.T) {
	manager := NewTokenManager(
		[]byte("12345678901234567890123456789012"),
		3600,
	)

	token, err := manager.Issue(42)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	called := false

	next := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		called = true

		userID, ok := UserIDFromContext(r.Context())
		if !ok {
			t.Fatal("user id missing from context")
		}

		if userID != 42 {
			t.Fatalf("expected user id 42, got %d", userID)
		}

		w.WriteHeader(http.StatusNoContent)
	})

	handler := manager.Middleware(next)

	req := httptest.NewRequest(
		http.MethodGet,
		"/api/v1/pins",
		nil,
	)

	req.Header.Set("Authorization", "Bearer "+token)

	rec := httptest.NewRecorder()

	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusNoContent {
		t.Fatalf("expected 204, got %d", rec.Code)
	}

	if !called {
		t.Fatal("next handler was not called")
	}
}

func TestMiddlewareMissingAuthorization(t *testing.T) {
	manager := NewTokenManager(
		[]byte("12345678901234567890123456789012"),
		3600,
	)

	next := http.HandlerFunc(func(
		http.ResponseWriter,
		*http.Request,
	) {
		t.Fatal("next handler must not be called")
	})

	req := httptest.NewRequest(
		http.MethodGet,
		"/",
		nil,
	)

	rec := httptest.NewRecorder()

	manager.Middleware(next).ServeHTTP(rec, req)

	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("expected 401, got %d", rec.Code)
	}
}

func TestMiddlewareInvalidToken(t *testing.T) {
	manager := NewTokenManager(
		[]byte("12345678901234567890123456789012"),
		3600,
	)

	req := httptest.NewRequest(
		http.MethodGet,
		"/",
		nil,
	)

	req.Header.Set(
		"Authorization",
		"Bearer invalid-token",
	)

	rec := httptest.NewRecorder()

	manager.Middleware(http.HandlerFunc(func(
		http.ResponseWriter,
		*http.Request,
	) {
		t.Fatal("next handler must not be called")
	})).ServeHTTP(rec, req)

	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("expected 401, got %d", rec.Code)
	}
}

func TestMiddlewareWrongSigningMethod(t *testing.T) {
	secret := []byte("12345678901234567890123456789012")

	manager := NewTokenManager(secret, 3600)

	claims := Claims{
		UserID: 42,
		RegisteredClaims: jwt.RegisteredClaims{
			Subject:   "42",
			IssuedAt:  jwt.NewNumericDate(time.Now()),
			ExpiresAt: jwt.NewNumericDate(time.Now().Add(time.Hour)),
		},
	}

	token := jwt.NewWithClaims(jwt.SigningMethodHS512, claims)

	tokenString, err := token.SignedString(secret)
	if err != nil {
		t.Fatal(err)
	}

	req := httptest.NewRequest(
		http.MethodGet,
		"/",
		nil,
	)

	req.Header.Set(
		"Authorization",
		"Bearer "+tokenString,
	)

	rec := httptest.NewRecorder()

	manager.Middleware(http.HandlerFunc(func(
		http.ResponseWriter,
		*http.Request,
	) {
		t.Fatal("next handler must not be called")
	})).ServeHTTP(rec, req)

	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("expected 401, got %d", rec.Code)
	}
}

func TestIssueInvalidUserID(t *testing.T) {
	manager := NewTokenManager(
		[]byte("12345678901234567890123456789012"),
		3600,
	)

	_, err := manager.Issue(0)

	if err == nil {
		t.Fatal("expected error")
	}
}

func TestAuthorizationHeaderFormat(t *testing.T) {
	manager := NewTokenManager(
		[]byte("12345678901234567890123456789012"),
		3600,
	)

	tests := []string{
		"",
		"Basic abc",
		"Bearer",
		"Bearer ",
		strings.Repeat("x", 100),
	}

	for _, authorization := range tests {
		t.Run(authorization, func(t *testing.T) {
			req := httptest.NewRequest(
				http.MethodGet,
				"/",
				nil,
			)

			if authorization != "" {
				req.Header.Set("Authorization", authorization)
			}

			rec := httptest.NewRecorder()

			manager.Middleware(
				http.HandlerFunc(func(
					http.ResponseWriter,
					*http.Request,
				) {
					t.Fatal("next handler must not be called")
				}),
			).ServeHTTP(rec, req)

			if rec.Code != http.StatusUnauthorized {
				t.Fatalf(
					"expected 401, got %d",
					rec.Code,
				)
			}
		})
	}
}
