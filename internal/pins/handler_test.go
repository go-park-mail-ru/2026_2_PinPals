package pins

import (
	"bytes"
	"encoding/json"
	"errors"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"2026_2_PinPals/internal/middleware"
	"2026_2_PinPals/internal/model"
)

func pinsTestLogger() *slog.Logger {
	return slog.New(slog.NewTextHandler(io.Discard, nil))
}

func TestListHandler(t *testing.T) {
	now := time.Date(2026, 10, 4, 0, 0, 0, 0, time.UTC)

	tests := []struct {
		name       string
		target     string
		repo       *fakePinRepository
		wantStatus int
	}{
		{
			name:   "default success",
			target: "/api/v1/pins",
			repo: &fakePinRepository{
				listResult: []model.Pin{{ID: 1, CreatedAt: now}},
			},
			wantStatus: http.StatusOK,
		},
		{
			name:       "invalid limit",
			target:     "/api/v1/pins?limit=nope",
			repo:       &fakePinRepository{},
			wantStatus: http.StatusBadRequest,
		},
		{
			name:       "invalid cursor",
			target:     "/api/v1/pins?cursor=bad",
			repo:       &fakePinRepository{},
			wantStatus: http.StatusBadRequest,
		},
		{
			name:       "service validation",
			target:     "/api/v1/pins?limit=0",
			repo:       &fakePinRepository{},
			wantStatus: http.StatusBadRequest,
		},
		{
			name:   "repository error",
			target: "/api/v1/pins",
			repo: &fakePinRepository{
				listErr: errors.New("database error"),
			},
			wantStatus: http.StatusInternalServerError,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			service := NewService(tt.repo)
			handler := NewHandler(service, pinsTestLogger())

			req := httptest.NewRequest(http.MethodGet, tt.target, nil)
			rec := httptest.NewRecorder()

			handler.List(rec, req)

			if rec.Code != tt.wantStatus {
				t.Fatalf("expected %d, got %d", tt.wantStatus, rec.Code)
			}
		})
	}
}

func TestListHandlerNextCursor(t *testing.T) {
	now := time.Date(2026, 10, 4, 0, 0, 0, 0, time.UTC)
	repo := &fakePinRepository{
		listResult: []model.Pin{
			{ID: 1, CreatedAt: now},
			{ID: 2, CreatedAt: now.Add(-time.Minute)},
			{ID: 3, CreatedAt: now.Add(-2 * time.Minute)},
		},
		listHasNext: true,
	}
	handler := NewHandler(NewService(repo), pinsTestLogger())

	req := httptest.NewRequest(http.MethodGet, "/api/v1/pins?limit=2", nil)
	rec := httptest.NewRecorder()
	handler.List(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", rec.Code)
	}
	var body struct {
		Pins       []model.Pin `json:"pins"`
		NextCursor string      `json:"next_cursor"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
		t.Fatal(err)
	}
	if len(body.Pins) != 2 || body.NextCursor == "" {
		t.Fatalf("unexpected response: %#v", body)
	}
}

func TestGetByIDHandler(t *testing.T) {
	tests := []struct {
		name       string
		pathValue  string
		repo       *fakePinRepository
		wantStatus int
	}{
		{"invalid id", "abc", &fakePinRepository{}, http.StatusBadRequest},
		{"validation", "0", &fakePinRepository{}, http.StatusBadRequest},
		{"not found", "42", &fakePinRepository{getErr: ErrPinNotFound}, http.StatusNotFound},
		{"repository error", "42", &fakePinRepository{getErr: errors.New("database error")}, http.StatusInternalServerError},
		{"success", "42", &fakePinRepository{getResult: model.Pin{ID: 42}}, http.StatusOK},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			handler := NewHandler(NewService(tt.repo), pinsTestLogger())
			req := httptest.NewRequest(http.MethodGet, "/api/v1/pins/"+tt.pathValue, nil)
			req.SetPathValue("pinID", tt.pathValue)
			rec := httptest.NewRecorder()

			handler.GetByID(rec, req)

			if rec.Code != tt.wantStatus {
				t.Fatalf("expected %d, got %d", tt.wantStatus, rec.Code)
			}
		})
	}
}

func TestCreateHandler(t *testing.T) {
	tests := []struct {
		name       string
		body       string
		repo       *fakePinRepository
		auth       bool
		wantStatus int
	}{
		{"unauthorized", `{"image_url":"x","name":"Beach"}`, &fakePinRepository{}, false, http.StatusUnauthorized},
		{"bad json", `{`, &fakePinRepository{}, true, http.StatusBadRequest},
		{"validation", `{"image_url":"","name":"Beach"}`, &fakePinRepository{}, true, http.StatusBadRequest},
		{"repository error", `{"image_url":"x","name":"Beach"}`, &fakePinRepository{createErr: errors.New("database error")}, true, http.StatusInternalServerError},
		{"success", `{"image_url":"x","name":"Beach"}`, &fakePinRepository{}, true, http.StatusCreated},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			handler := NewHandler(NewService(tt.repo), pinsTestLogger())

			req := httptest.NewRequest(
				http.MethodPost,
				"/api/v1/pins",
				bytes.NewBufferString(tt.body),
			)
			req.Header.Set("Content-Type", "application/json")

			if tt.auth {
				manager := middleware.NewTokenManager(
					[]byte("12345678901234567890123456789012"),
					3600,
				)
				token, err := manager.Issue(42)
				if err != nil {
					t.Fatal(err)
				}
				req.Header.Set("Authorization", "Bearer "+token)

				rec := httptest.NewRecorder()
				manager.Middleware(http.HandlerFunc(handler.Create)).ServeHTTP(rec, req)

				if rec.Code != tt.wantStatus {
					t.Fatalf("expected %d, got %d", tt.wantStatus, rec.Code)
				}
				return
			}

			rec := httptest.NewRecorder()
			handler.Create(rec, req)

			if rec.Code != tt.wantStatus {
				t.Fatalf("expected %d, got %d", tt.wantStatus, rec.Code)
			}
		})
	}
}

func TestListHandlerWithCursor(t *testing.T) {
	cursor := Cursor{
		CreatedAt: time.Date(2026, 10, 4, 0, 0, 0, 0, time.UTC),
		ID:        10,
	}
	encoded, err := encodeCursor(cursor)
	if err != nil {
		t.Fatal(err)
	}

	repo := &fakePinRepository{
		listResult: []model.Pin{{ID: 9, CreatedAt: cursor.CreatedAt.Add(-time.Minute)}},
	}
	handler := NewHandler(NewService(repo), pinsTestLogger())

	req := httptest.NewRequest(
		http.MethodGet,
		"/api/v1/pins?cursor="+strings.TrimSpace(encoded),
		nil,
	)
	rec := httptest.NewRecorder()

	handler.List(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", rec.Code)
	}
}
