package auth

import (
	"bytes"
	"context"
	"errors"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"2026_2_PinPals/internal/model"
)

type fakeAuthHandlerService struct {
	registerResult model.User
	registerErr    error
	loginResult    model.AuthResponse
	loginErr       error
}

func (f *fakeAuthHandlerService) Register(
	_ context.Context,
	_ model.RegisterInput,
) (model.User, error) {
	if f.registerErr != nil {
		return model.User{}, f.registerErr
	}
	return f.registerResult, nil
}

func (f *fakeAuthHandlerService) Login(
	_ context.Context,
	_ model.LoginInput,
	_ func(int) (string, error),
) (model.AuthResponse, error) {
	if f.loginErr != nil {
		return model.AuthResponse{}, f.loginErr
	}
	return f.loginResult, nil
}

func testLogger() *slog.Logger {
	return slog.New(slog.NewTextHandler(io.Discard, nil))
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
			err:        ErrValidation,
			wantStatus: http.StatusBadRequest,
		},
		{
			name:       "conflict",
			body:       `{"name":"Alex"}`,
			err:        ErrUserTagExists,
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
				registerResult: model.User{ID: 1, Name: "Alex"},
				registerErr:    tt.err,
			}
			handler := NewHandler(service, nil, testLogger())

			req := httptest.NewRequest(http.MethodPost, "/api/v1/auth/register", bytes.NewBufferString(tt.body))
			req.Header.Set("Content-Type", "application/json")
			rec := httptest.NewRecorder()

			handler.Register(rec, req)

			if rec.Code != tt.wantStatus {
				t.Fatalf("expected %d, got %d", tt.wantStatus, rec.Code)
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
			body:       `{"user_tag":"","password":""}`,
			err:        ErrValidation,
			wantStatus: http.StatusUnauthorized,
		},
		{
			name:       "invalid credentials",
			body:       `{"user_tag":"alex","password":"wrong"}`,
			err:        ErrInvalidCredentials,
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
				loginResult: model.AuthResponse{
					Token: "jwt",
					User:  model.User{ID: 1, UserTag: "alex"},
				},
				loginErr: tt.err,
			}
			handler := NewHandler(
				service,
				func(int) (string, error) { return "jwt", nil },
				testLogger(),
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
	h := NewHandler(service, nil, testLogger())
	if h.service != service {
		t.Fatal("service was not stored")
	}
}

func TestAuthJSONBirthDateCompatibility(t *testing.T) {
	service := &fakeAuthHandlerService{
		registerResult: model.User{BirthDate: time.Date(2004, 7, 21, 0, 0, 0, 0, time.UTC)},
	}
	h := NewHandler(service, nil, testLogger())

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
