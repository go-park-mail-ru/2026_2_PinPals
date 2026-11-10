package http

import (
	"bytes"
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	httpmocks "2026_2_PinPals/auth_service/delivery/http/mocks"
	"2026_2_PinPals/auth_service/domain"
	"go.uber.org/mock/gomock"
)

type fakeAuthHandlerService struct {
	registerResult domain.User
	registerErr    error
	loginResult    domain.AuthResponse
	loginErr       error
}

func (f *fakeAuthHandlerService) Register(
	_ context.Context,
	_ domain.RegisterInput,
) (domain.User, error) {
	if f.registerErr != nil {
		return domain.User{}, f.registerErr
	}
	return f.registerResult, nil
}

func (f *fakeAuthHandlerService) Login(
	_ context.Context,
	_ domain.LoginInput,
	_ func(int) (string, error),
) (domain.AuthResponse, error) {
	if f.loginErr != nil {
		return domain.AuthResponse{}, f.loginErr
	}
	return f.loginResult, nil
}

func TestRegisterHandler(t *testing.T) {
	tests := []struct {
		name       string
		body       string
		err        error
		wantStatus int
	}{
		{
			name:       "success",
			body:       `{"name":"Alex","user_tag":"alex","birth_date":"2004-07-21T00:00:00Z","password":"password123"}`,
			wantStatus: http.StatusCreated,
		},
		{
			name:       "validation",
			body:       `{"name":"Alex"}`,
			err:        &domain.ValidationErrors{Fields: map[string]string{"user_tag": domain.ValidationRequired}},
			wantStatus: http.StatusBadRequest,
		},
		{
			name:       "conflict",
			body:       `{"name":"Alex"}`,
			err:        domain.ErrUserTagExists,
			wantStatus: http.StatusConflict,
		},
		{
			name:       "internal",
			body:       `{"name":"Alex"}`,
			err:        errors.New("database error"),
			wantStatus: http.StatusInternalServerError,
		},
		{
			name:       "bad json",
			body:       `{`,
			wantStatus: http.StatusBadRequest,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			service := &fakeAuthHandlerService{
				registerResult: domain.User{ID: 1, Name: "Alex"},
				registerErr:    tt.err,
			}
			handler := NewHandler(service, nil)

			req := httptest.NewRequest(http.MethodPost, "/api/v1/auth/register", bytes.NewBufferString(tt.body))
			req.Header.Set("Content-Type", "application/json")
			rec := httptest.NewRecorder()

			handler.Register(rec, req)

			if rec.Code != tt.wantStatus {
				t.Fatalf("expected %d, got %d", tt.wantStatus, rec.Code)
			}
			if tt.name == "validation" && !bytes.Contains(rec.Body.Bytes(), []byte("ошибка_валидации")) {
				t.Fatalf("expected localized validation code, got %s", rec.Body.String())
			}
		})
	}
}

func TestLoginHandler(t *testing.T) {
	tests := []struct {
		name       string
		body       string
		err        error
		wantStatus int
	}{
		{
			name:       "success",
			body:       `{"user_tag":"alex","password":"password123"}`,
			wantStatus: http.StatusOK,
		},
		{
			name:       "validation",
			body:       `{"name":"Alex"}`,
			err:        &domain.ValidationErrors{Fields: map[string]string{"user_tag": domain.ValidationRequired}},
			wantStatus: http.StatusBadRequest,
		},
		{
			name:       "empty credentials",
			body:       `{"user_tag":"","password":""}`,
			err:        domain.ErrValidation,
			wantStatus: http.StatusBadRequest,
		},
		{
			name:       "invalid credentials",
			body:       `{"user_tag":"alex","password":"wrong"}`,
			err:        domain.ErrInvalidCredentials,
			wantStatus: http.StatusUnauthorized,
		},
		{
			name:       "internal",
			body:       `{"user_tag":"alex","password":"password123"}`,
			err:        errors.New("database error"),
			wantStatus: http.StatusInternalServerError,
		},
		{
			name:       "bad json",
			body:       `{`,
			wantStatus: http.StatusBadRequest,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			service := &fakeAuthHandlerService{
				loginResult: domain.AuthResponse{
					Token: "jwt",
					User:  domain.User{ID: 1, UserTag: "alex"},
				},
				loginErr: tt.err,
			}
			handler := NewHandler(
				service,
				func(int) (string, error) { return "jwt", nil },
			)

			req := httptest.NewRequest(http.MethodPost, "/api/v1/auth/login", bytes.NewBufferString(tt.body))
			req.Header.Set("Content-Type", "application/json")
			rec := httptest.NewRecorder()

			handler.Login(rec, req)

			if rec.Code != tt.wantStatus {
				t.Fatalf("expected %d, got %d", tt.wantStatus, rec.Code)
			}
		})
	}
}

func TestNewHandler(t *testing.T) {
	service := &fakeAuthHandlerService{}
	h := NewHandler(service, nil)
	if h.service != service {
		t.Fatal("service was not stored")
	}
}

func TestAuthJSONBirthDateCompatibility(t *testing.T) {
	service := &fakeAuthHandlerService{
		registerResult: domain.User{BirthDate: time.Date(2004, 7, 21, 0, 0, 0, 0, time.UTC)},
	}
	h := NewHandler(service, nil)

	req := httptest.NewRequest(
		http.MethodPost,
		"/api/v1/auth/register",
		bytes.NewBufferString(`{"name":"Alex","user_tag":"alex","birth_date":"2004-07-21T00:00:00Z","password":"password123"}`),
	)
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()

	h.Register(rec, req)
	if rec.Code != http.StatusCreated {
		t.Fatalf("expected 201, got %d", rec.Code)
	}
}

func TestLoginSetsCookiesAndDoesNotExposeToken(t *testing.T) {
	service := &fakeAuthHandlerService{
		loginResult: domain.AuthResponse{
			Token: "private-jwt-value",
			User:  domain.User{ID: 7, UserTag: "alex"},
		},
	}
	h := NewHandler(service, func(int) (string, error) { return "private-jwt-value", nil }, HandlerOptions{TokenTTL: 2 * time.Hour, SecureCookies: true})
	req := httptest.NewRequest(http.MethodPost, "/api/v1/auth/login", bytes.NewBufferString(`{"user_tag":"alex","password":"password123"}`))
	req.Header.Set("Content-Type", "application/json; charset=utf-8")
	rec := httptest.NewRecorder()

	h.Login(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", rec.Code, rec.Body.String())
	}
	if bytes.Contains(rec.Body.Bytes(), []byte("private-jwt-value")) || bytes.Contains(rec.Body.Bytes(), []byte(`"token"`)) {
		t.Fatalf("JWT must not be present in response body: %s", rec.Body.String())
	}
	cookies := rec.Result().Cookies()
	var access, csrf *http.Cookie
	for _, cookie := range cookies {
		switch cookie.Name {
		case "access_token":
			access = cookie
		case "csrf_token":
			csrf = cookie
		}
	}
	if access == nil || !access.HttpOnly || !access.Secure || access.Value != "private-jwt-value" || access.MaxAge != 7200 {
		t.Fatalf("invalid access cookie: %#v", access)
	}
	if csrf == nil || csrf.HttpOnly || !csrf.Secure || csrf.Value == "" || csrf.MaxAge != 7200 {
		t.Fatalf("invalid CSRF cookie: %#v", csrf)
	}
}

func TestLogoutRevokesTokenAndClearsCookies(t *testing.T) {
	ctrl := gomock.NewController(t)
	revoker := httpmocks.NewMockTokenRevoker(ctrl)
	revoker.EXPECT().Revoke(gomock.Any(), "jwt-to-revoke").Return(nil)
	h := NewHandler(&fakeAuthHandlerService{}, nil, HandlerOptions{TokenRevoker: revoker})
	req := httptest.NewRequest(http.MethodPost, "/api/v1/auth/logout", nil)
	req.AddCookie(&http.Cookie{Name: "access_token", Value: "jwt-to-revoke"})
	rec := httptest.NewRecorder()

	h.Logout(rec, req)

	if rec.Code != http.StatusNoContent {
		t.Fatalf("expected 204, got %d: %s", rec.Code, rec.Body.String())
	}
	cleared := map[string]bool{}
	for _, cookie := range rec.Result().Cookies() {
		if cookie.MaxAge < 0 && cookie.Value == "" {
			cleared[cookie.Name] = true
		}
	}
	if !cleared["access_token"] || !cleared["csrf_token"] {
		t.Fatalf("expected both auth cookies to be cleared, got %#v", cleared)
	}
}

func TestLogoutReportsRevocationFailure(t *testing.T) {
	ctrl := gomock.NewController(t)
	revoker := httpmocks.NewMockTokenRevoker(ctrl)
	revoker.EXPECT().Revoke(gomock.Any(), "jwt-to-revoke").Return(errors.New("database unavailable"))
	h := NewHandler(&fakeAuthHandlerService{}, nil, HandlerOptions{TokenRevoker: revoker})
	req := httptest.NewRequest(http.MethodPost, "/api/v1/auth/logout", nil)
	req.AddCookie(&http.Cookie{Name: "access_token", Value: "jwt-to-revoke"})
	rec := httptest.NewRecorder()

	h.Logout(rec, req)

	if rec.Code != http.StatusInternalServerError {
		t.Fatalf("expected 500, got %d", rec.Code)
	}
	if len(rec.Result().Cookies()) != 0 {
		t.Fatal("cookies must not be cleared when token revocation failed")
	}
}
