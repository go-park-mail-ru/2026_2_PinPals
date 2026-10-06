package server

import (
	"bytes"
	"context"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"testing"

	"2026_2_PinPals/internal/auth"
	"2026_2_PinPals/internal/config"
	"2026_2_PinPals/internal/middleware"
	"2026_2_PinPals/internal/model"
	"2026_2_PinPals/internal/pins"
)

type fakeServerAuthService struct{}

func (fakeServerAuthService) Register(context.Context, model.RegisterInput) (model.User, error) {
	return model.User{ID: 1, Name: "Alex", UserTag: "alex"}, nil
}

func (fakeServerAuthService) Login(
	context.Context,
	model.LoginInput,
	func(int) (string, error),
) (model.AuthResponse, error) {
	return model.AuthResponse{Token: "jwt", User: model.User{ID: 1}}, nil
}

type fakeServerPinRepository struct{}

func (fakeServerPinRepository) Create(
	_ context.Context,
	creatorID int,
	input model.CreatePinInput,
) (model.Pin, error) {
	return model.Pin{
		ID:        1,
		CreatorID: creatorID,
		ImageURL:  input.ImageURL,
		Name:      input.Name,
	}, nil
}

func (fakeServerPinRepository) List(
	_ context.Context,
	_ int,
	_ *pins.Cursor,
) ([]model.Pin, bool, error) {
	return []model.Pin{{ID: 1, Name: "Beach"}}, false, nil
}

func (fakeServerPinRepository) GetByID(context.Context, int) (model.Pin, error) {
	return model.Pin{ID: 1, Name: "Beach"}, nil
}

func TestNewRouter(t *testing.T) {
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	authHandler := auth.NewHandler(
		fakeServerAuthService{},
		func(int) (string, error) { return "jwt", nil },
		logger,
	)
	pinHandler := pins.NewHandler(
		pins.NewService(fakeServerPinRepository{}),
		logger,
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
