package http

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"io"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/go-playground/validator/v10"

	"2026_2_PinPals/pin_service/domain"
	"2026_2_PinPals/pin_service/usecase"
	"2026_2_PinPals/pkg/images"
	"2026_2_PinPals/pkg/middleware"
)

type fakePinRepository struct {
	createResult domain.Pin
	createErr    error

	listResult  []domain.Pin
	listHasNext bool
	listErr     error

	getResult domain.Pin
	getErr    error
}

func (f *fakePinRepository) Create(
	_ context.Context,
	creatorID int,
	input domain.CreatePinInput,
) (domain.Pin, error) {
	if f.createErr != nil {
		return domain.Pin{}, f.createErr
	}

	return domain.Pin{
		ID:          1,
		CreatorID:   creatorID,
		ImageURL:    input.ImageURL,
		Name:        input.Name,
		Description: input.Description,
	}, nil
}

func (f *fakePinRepository) List(
	_ context.Context,
	_ int,
	_ *domain.Cursor,
) ([]domain.Pin, bool, error) {
	if f.listErr != nil {
		return nil, false, f.listErr
	}

	return f.listResult, f.listHasNext, nil
}

func (f *fakePinRepository) GetByID(
	_ context.Context,
	_ int,
) (domain.Pin, error) {
	if f.getErr != nil {
		return domain.Pin{}, f.getErr
	}

	return f.getResult, nil
}

// Создаёт Handler для тестов с тестовым валидатором и временным хранилищем.
func newPinsTestHandler(t *testing.T, service PinService) *Handler {
	t.Helper()
	dir := t.TempDir()
	return NewHandler(service, "", validator.New(), images.NewStorage(dir))
}

func newMultipartPinRequest(t *testing.T, name string) *http.Request {
	t.Helper()
	var body bytes.Buffer
	writer := multipart.NewWriter(&body)
	if err := writer.WriteField("name", name); err != nil {
		t.Fatal(err)
	}
	part, err := writer.CreateFormFile("image", "upload.png")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := part.Write([]byte("\x89PNG\r\n\x1a\ncontent")); err != nil {
		t.Fatal(err)
	}
	if err := writer.Close(); err != nil {
		t.Fatal(err)
	}
	req := httptest.NewRequest(http.MethodPost, "/api/v1/pins", &body)
	req.Header.Set("Content-Type", writer.FormDataContentType())
	return req
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
				listResult: []domain.Pin{{ID: 1, CreatedAt: now}},
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
			service := usecase.NewService(tt.repo)
			handler := newPinsTestHandler(t, service)

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
		listResult: []domain.Pin{
			{ID: 1, CreatedAt: now},
			{ID: 2, CreatedAt: now.Add(-time.Minute)},
			{ID: 3, CreatedAt: now.Add(-2 * time.Minute)},
		},
		listHasNext: true,
	}

	handler := newPinsTestHandler(t, usecase.NewService(repo))

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

	var body ListResponse
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
		{"not found", "42", &fakePinRepository{getErr: domain.ErrPinNotFound}, http.StatusNotFound},
		{"repository error", "42", &fakePinRepository{getErr: errors.New("database error")}, http.StatusInternalServerError},
		{"success", "42", &fakePinRepository{getResult: domain.Pin{ID: 42}}, http.StatusOK},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			handler := newPinsTestHandler(t, usecase.NewService(tt.repo))

			req := httptest.NewRequest(
				http.MethodGet,
				"/api/v1/pins/"+tt.pathValue,
				nil,
			)
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
		{"unauthorized", "Beach", &fakePinRepository{}, false, http.StatusUnauthorized},
		{"missing image", "Beach", &fakePinRepository{}, true, http.StatusBadRequest},
		{"validation", "", &fakePinRepository{}, true, http.StatusBadRequest},
		{"repository error", "Beach", &fakePinRepository{createErr: errors.New("database error")}, true, http.StatusInternalServerError},
		{"success", "Beach", &fakePinRepository{}, true, http.StatusCreated},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			handler := newPinsTestHandler(t, usecase.NewService(tt.repo))

			var req *http.Request
			if tt.name == "missing image" || tt.name == "validation" {
				var body bytes.Buffer
				writer := multipart.NewWriter(&body)
				_ = writer.WriteField("name", tt.body)
				_ = writer.Close()
				req = httptest.NewRequest(http.MethodPost, "/api/v1/pins", &body)
				req.Header.Set("Content-Type", writer.FormDataContentType())
			} else {
				req = newMultipartPinRequest(t, tt.body)
			}

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

				manager.Middleware(
					http.HandlerFunc(handler.Create),
				).ServeHTTP(rec, req)

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
	cursor := domain.Cursor{
		CreatedAt: time.Date(2026, 10, 4, 0, 0, 0, 0, time.UTC),
		ID:        10,
	}

	repo := &fakePinRepository{
		listResult: []domain.Pin{
			{ID: 9, CreatedAt: cursor.CreatedAt.Add(-time.Minute)},
		},
	}

	handler := newPinsTestHandler(t, usecase.NewService(repo))

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

type failingImageStorage struct{}

func (failingImageStorage) Save(io.Reader) (string, error) { return "", errors.New("disk unavailable") }
func (failingImageStorage) Delete(string) error            { return nil }

func TestCreateHandlerMapsImageStorageFailureToInternalServerError(t *testing.T) {
	service := usecase.NewService(&fakePinRepository{})
	handler := NewHandler(service, "", validator.New(), failingImageStorage{})
	req := newMultipartPinRequest(t, "Beach")
	manager := middleware.NewTokenManager([]byte("12345678901234567890123456789012"), 3600)
	token, err := manager.Issue(42)
	if err != nil {
		t.Fatal(err)
	}
	req.Header.Set("Authorization", "Bearer "+token)
	rec := httptest.NewRecorder()
	manager.Middleware(http.HandlerFunc(handler.Create)).ServeHTTP(rec, req)
	if rec.Code != http.StatusInternalServerError {
		t.Fatalf("expected 500 for storage failure, got %d: %s", rec.Code, rec.Body.String())
	}
}
