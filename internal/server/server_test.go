package server

import (
	"bytes"
	"context"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"testing"

	authdelivery "2026_2_PinPals/auth_service/delivery"
	authdomain "2026_2_PinPals/auth_service/domain"
	"2026_2_PinPals/internal/config"
	"2026_2_PinPals/internal/middleware"
	pindelivery "2026_2_PinPals/pin_service/delivery"
	pindomain "2026_2_PinPals/pin_service/domain"
	"2026_2_PinPals/pin_service/usecase"
)

type fakeServerAuthService struct{}

func (fakeServerAuthService) Register(context.Context, authdomain.RegisterInput) (authdomain.User, error) {
	return authdomain.User{ID: 1, Name: "Alex", UserTag: "alex"}, nil
}

func (fakeServerAuthService) Login(
	context.Context,
	authdomain.LoginInput,
	func(int) (string, error),
) (authdomain.AuthResponse, error) {
	return authdomain.AuthResponse{Token: "jwt", User: authdomain.User{ID: 1}}, nil
}

type fakeServerPinRepository struct{}

func (fakeServerPinRepository) Create(
	_ context.Context,
	creatorID int,
	input pindomain.CreatePinInput,
) (pindomain.Pin, error) {
	return pindomain.Pin{
		ID:        1,
		CreatorID: creatorID,
		ImageURL:  input.ImageURL,
		Name:      input.Name,
	}, nil
}

func (fakeServerPinRepository) List(
	_ context.Context,
	_ int,
	_ *pindomain.Cursor,
) ([]pindomain.Pin, bool, error) {
	return []pindomain.Pin{{ID: 1, Name: "Beach"}}, false, nil
}

func (fakeServerPinRepository) GetByID(context.Context, int) (pindomain.Pin, error) {
	return pindomain.Pin{ID: 1, Name: "Beach"}, nil
}

func TestNewRouter(t *testing.T) {
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	authHandler := authdelivery.NewHandler(
		fakeServerAuthService{},
		func(int) (string, error) { return "jwt", nil },
		logger,
	)
	pinHandler := pindelivery.NewHandler(
		usecase.NewService(fakeServerPinRepository{}),
		logger,
		"",
	)
	tokenManager := middleware.NewTokenManager(
		[]byte("12345678901234567890123456789012"),
		3600,
	)

	cfg := &config.Config{
		CORS: config.CORSConfig{
			AllowedOrigins: []string{"http://localhost:3000"},
		},
	}

	handler := NewRouter(cfg, logger, authHandler, pinHandler, nil, tokenManager)

	t.Run("health", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/healthz", nil)
		rec := httptest.NewRecorder()
		handler.ServeHTTP(rec, req)

		if rec.Code != http.StatusOK {
			t.Fatalf("expected 200, got %d", rec.Code)
		}
	})

	t.Run("cors preflight", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodOptions, "/healthz", nil)
		req.Header.Set("Origin", "http://localhost:3000")
		rec := httptest.NewRecorder()
		handler.ServeHTTP(rec, req)

		if rec.Code != http.StatusNoContent {
			t.Fatalf("expected 204, got %d", rec.Code)
		}
	})

	t.Run("pin list", func(t *testing.T) {
		req := httptest.NewRequest(
			http.MethodPost,
			"/api/v1/pins/search",
			bytes.NewBufferString(`{}`),
		)
		req.Header.Set("Content-Type", "application/json")

		rec := httptest.NewRecorder()
		handler.ServeHTTP(rec, req)
		if rec.Code != http.StatusOK {
			t.Fatalf("expected 200, got %d", rec.Code)
		}
	})

	t.Run("pin create unauthorized", func(t *testing.T) {
		req := httptest.NewRequest(
			http.MethodPost,
			"/api/v1/pins",
			bytes.NewBufferString(`{"image_url":"x.jpg","name":"Beach"}`),
		)
		req.Header.Set("Content-Type", "application/json")
		rec := httptest.NewRecorder()
		handler.ServeHTTP(rec, req)

		if rec.Code != http.StatusUnauthorized {
			t.Fatalf("expected 401, got %d", rec.Code)
		}
	})
}
