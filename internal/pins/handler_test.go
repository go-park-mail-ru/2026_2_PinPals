package pins

import (
	"bytes"
	"encoding/json"
	"errors"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
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
		body       string
		repo       *fakePinRepository
		wantStatus int
	}{
		{
			name:   "default success",
			target: "/api/v1/pins/search",
			body:   `{}`,
			repo: &fakePinRepository{
				listResult: []model.Pin{{ID: 1, CreatedAt: now}},
			},
			wantStatus: http.StatusOK,
		},
		{
			name:       "invalid limit",
			target:     "/api/v1/pins/search",
			body:       `{"limit":0}`,
			repo:       &fakePinRepository{},
			wantStatus: http.StatusBadRequest,
		},
		{
			name:       "invalid cursor",
			target:     "/api/v1/pins/search",
			body:       `{"cursor":{"id":0,"created_at":"2026-10-04T00:00:00Z"}}`,
			repo:       &fakePinRepository{},
			wantStatus: http.StatusBadRequest,
		},
		{
			name:       "service validation",
			target:     "/api/v1/pins/search",
			body:       `{"limit":0}`,
			repo:       &fakePinRepository{},
			wantStatus: http.StatusBadRequest,
		},
		{
			name:   "repository error",
			target: "/api/v1/pins/search",
			body:   `{}`,
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

			req := httptest.NewRequest(
				http.MethodPost,
				tt.target,
				bytes.NewBufferString(tt.body),
			)
			req.Header.Set("Content-Type", "application/json")

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

	req := httptest.NewRequest(
		http.MethodPost,
		"/api/v1/pins/search",
		bytes.NewBufferString(`{"limit":2}`),
	)
	req.Header.Set("Content-Type", "application/json")

	rec := httptest.NewRecorder()
	handler.List(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", rec.Code)
	}
	var body struct {
		Pins       []model.Pin `json:"pins"`
		NextCursor *Cursor     `json:"next_cursor"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
		t.Fatal(err)
	}

	if len(body.Pins) != 2 {
		t.Fatalf("expected 2 pins, got %d", len(body.Pins))
	}

	if body.NextCursor == nil {
		t.Fatal("expected next cursor")
	}

	if body.NextCursor.ID != 2 {
		t.Fatalf("expected next cursor id 2, got %d", body.NextCursor.ID)
	}

	if !body.NextCursor.CreatedAt.Equal(now.Add(-time.Minute)) {
		t.Fatalf("unexpected next cursor timestamp: %v", body.NextCursor.CreatedAt)
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

func TestGetByIDHandlerUsesConfiguredBaseURL(t *testing.T) {
	handler := NewHandler(
		NewService(&fakePinRepository{
			getResult: model.Pin{ID: 42, ImageURL: "1.png"},
		}),
		pinsTestLogger(),
		"https://api.example.com",
	)

	req := httptest.NewRequest(http.MethodGet, "/api/v1/pins/42", nil)
	req.Host = "internal:8080"
	req.SetPathValue("pinID", "42")
	rec := httptest.NewRecorder()

	handler.GetByID(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", rec.Code)
	}

	var body model.Pin
	if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
		t.Fatal(err)
	}

	if body.ImageURL != "https://api.example.com/images/1.png" {
		t.Fatalf("unexpected image url: %q", body.ImageURL)
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
		{"unauthorized", `{"image_url":"x.png","name":"Beach"}`, &fakePinRepository{}, false, http.StatusUnauthorized},
		{"bad json", `{`, &fakePinRepository{}, true, http.StatusBadRequest},
		{"validation", `{"image_url":"","name":"Beach"}`, &fakePinRepository{}, true, http.StatusBadRequest},
		{"repository error", `{"image_url":"x.png","name":"Beach"}`, &fakePinRepository{createErr: errors.New("database error")}, true, http.StatusInternalServerError},
		{"success", `{"image_url":"x.png","name":"Beach"}`, &fakePinRepository{}, true, http.StatusCreated},
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

	repo := &fakePinRepository{
		listResult: []model.Pin{{ID: 9, CreatedAt: cursor.CreatedAt.Add(-time.Minute)}},
	}
	handler := NewHandler(NewService(repo), pinsTestLogger())

	req := httptest.NewRequest(
		http.MethodPost,
		"/api/v1/pins/search",
		bytes.NewBufferString(`{"cursor": {"created_at": "2026-10-04T00:00:00Z","id": 10}}`),
	)
	req.Header.Set("Content-Type", "application/json")

	rec := httptest.NewRecorder()

	handler.List(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", rec.Code)
	}
}
