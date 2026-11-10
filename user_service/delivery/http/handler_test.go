package http

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"2026_2_PinPals/pkg/images"
	"2026_2_PinPals/pkg/middleware"
	"2026_2_PinPals/user_service/domain"
)

type fakeUserService struct {
	user        *domain.User
	getErr      error
	updateErr   error
	deleteErr   error
	updated     domain.UpdateProfileInput
	deleted     bool
	updatedID   int
	requestedID int
}

func (f *fakeUserService) GetProfile(_ context.Context, id int) (*domain.User, error) {
	f.requestedID = id
	return f.user, f.getErr
}
func (f *fakeUserService) UpdateProfile(_ context.Context, id int, input domain.UpdateProfileInput) (*domain.User, error) {
	f.updatedID = id
	f.updated = input
	if f.updateErr == nil && f.user != nil && input.AvatarURLSet {
		f.user.AvatarURL = input.AvatarURL
	}
	return f.user, f.updateErr
}
func (f *fakeUserService) DeleteProfile(_ context.Context, id int) error {
	f.requestedID = id
	f.deleted = true
	return f.deleteErr
}

func serveAsUser(t *testing.T, userID int, next http.Handler, method, target string, body string) *httptest.ResponseRecorder {
	t.Helper()
	manager := middleware.NewTokenManager([]byte("01234567890123456789012345678901"), int64(time.Hour/time.Second))
	token, err := manager.Issue(userID)
	if err != nil {
		t.Fatal(err)
	}
	req := httptest.NewRequest(method, target, strings.NewReader(body))
	req.Header.Set("Authorization", "Bearer "+token)
	if body != "" {
		req.Header.Set("Content-Type", "application/json")
	}
	rec := httptest.NewRecorder()
	manager.Middleware(next).ServeHTTP(rec, req)
	return rec
}

func TestGetMeReturnsCurrentUser(t *testing.T) {
	service := &fakeUserService{user: &domain.User{ID: 9, Name: "Alex", UserTag: "alex"}}
	h := NewHandler(service)
	rec := serveAsUser(t, 9, http.HandlerFunc(h.GetMe), http.MethodGet, "/api/v1/users/me", "")
	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", rec.Code, rec.Body.String())
	}
	var got UserResponse
	if err := json.Unmarshal(rec.Body.Bytes(), &got); err != nil {
		t.Fatal(err)
	}
	if got.ID != 9 || service.requestedID != 9 {
		t.Fatalf("unexpected user response/request: %#v, %d", got, service.requestedID)
	}
}

func TestUpdateMeCanClearDescription(t *testing.T) {
	service := &fakeUserService{user: &domain.User{ID: 9, Name: "Alex", UserTag: "alex"}}
	h := NewHandler(service)
	rec := serveAsUser(t, 9, http.HandlerFunc(h.UpdateMe), http.MethodPatch, "/api/v1/users/me", `{"description":null}`)
	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", rec.Code, rec.Body.String())
	}
	if service.updatedID != 9 || !service.updated.DescriptionSet || service.updated.AvatarURLSet {
		t.Fatalf("unexpected update input: %#v", service.updated)
	}
	if service.updated.Description != nil {
		t.Fatalf("expected description to be cleared: %#v", service.updated.Description)
	}
}

func TestUpdateMeRejectsClientSuppliedAvatarURL(t *testing.T) {
	service := &fakeUserService{user: &domain.User{ID: 9}}
	h := NewHandler(service)
	rec := serveAsUser(t, 9, http.HandlerFunc(h.UpdateMe), http.MethodPatch, "/api/v1/users/me", `{"avatar_url":"https://example.com/x.png"}`)
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d: %s", rec.Code, rec.Body.String())
	}
}

func TestUpdateMeRejectsInvalidNullableValue(t *testing.T) {
	service := &fakeUserService{user: &domain.User{ID: 9}}
	h := NewHandler(service)
	rec := serveAsUser(t, 9, http.HandlerFunc(h.UpdateMe), http.MethodPatch, "/api/v1/users/me", `{"description":42}`)
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d", rec.Code)
	}
}

func TestDeleteMeCallsService(t *testing.T) {
	service := &fakeUserService{}
	h := NewHandler(service)
	rec := serveAsUser(t, 9, http.HandlerFunc(h.DeleteMe), http.MethodDelete, "/api/v1/users/me", "")
	if rec.Code != http.StatusNoContent || !service.deleted || service.requestedID != 9 {
		t.Fatalf("unexpected delete result: status=%d deleted=%v id=%d", rec.Code, service.deleted, service.requestedID)
	}
}

func TestGetMeMapsNotFound(t *testing.T) {
	service := &fakeUserService{getErr: domain.ErrUserNotFound}
	h := NewHandler(service)
	rec := serveAsUser(t, 9, http.HandlerFunc(h.GetMe), http.MethodGet, "/api/v1/users/me", "")
	if rec.Code != http.StatusNotFound {
		t.Fatalf("expected 404, got %d", rec.Code)
	}
}

func TestDeleteMeMapsInternalError(t *testing.T) {
	service := &fakeUserService{deleteErr: errors.New("database unavailable")}
	h := NewHandler(service)
	rec := serveAsUser(t, 9, http.HandlerFunc(h.DeleteMe), http.MethodDelete, "/api/v1/users/me", "")
	if rec.Code != http.StatusInternalServerError {
		t.Fatalf("expected 500, got %d", rec.Code)
	}
}

func TestUpdateMeUploadsAvatarAsMultipartImage(t *testing.T) {
	service := &fakeUserService{user: &domain.User{ID: 9, Name: "Alex", UserTag: "alex"}}
	storageDir := t.TempDir()
	h := NewHandler(service, HandlerOptions{Storage: images.NewStorage(storageDir), ImageBaseURL: "http://localhost:8002"})
	body := &bytes.Buffer{}
	writer := multipart.NewWriter(body)
	if err := writer.WriteField("name", "Alex Updated"); err != nil {
		t.Fatal(err)
	}
	part, err := writer.CreateFormFile("avatar", "avatar.png")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := part.Write([]byte("\x89PNG\r\n\x1a\nexample-image")); err != nil {
		t.Fatal(err)
	}
	if err := writer.Close(); err != nil {
		t.Fatal(err)
	}

	manager := middleware.NewTokenManager([]byte("01234567890123456789012345678901"), int64(time.Hour/time.Second))
	token, err := manager.Issue(9)
	if err != nil {
		t.Fatal(err)
	}
	req := httptest.NewRequest(http.MethodPatch, "/api/v1/users/me", body)
	req.Header.Set("Authorization", "Bearer "+token)
	req.Header.Set("Content-Type", writer.FormDataContentType())
	rec := httptest.NewRecorder()
	manager.Middleware(http.HandlerFunc(h.UpdateMe)).ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", rec.Code, rec.Body.String())
	}
	if !service.updated.AvatarURLSet || service.updated.AvatarURL == nil {
		t.Fatalf("avatar not saved into update input: %#v", service.updated)
	}
	if filepath.Ext(*service.updated.AvatarURL) != ".png" || !strings.Contains(*service.updated.AvatarURL, "-") {
		t.Fatalf("expected UUID image filename, got %q", *service.updated.AvatarURL)
	}
	if _, err := os.Stat(filepath.Join(storageDir, *service.updated.AvatarURL)); err != nil {
		t.Fatalf("uploaded image was not stored: %v", err)
	}
	if !strings.Contains(rec.Body.String(), "http://localhost:8002/images/") {
		t.Fatalf("response avatar_url is not public: %s", rec.Body.String())
	}
}

func TestUpdateMeCleansUploadedAvatarWhenProfileUpdateFails(t *testing.T) {
	service := &fakeUserService{user: &domain.User{ID: 9}, updateErr: errors.New("database unavailable")}
	storageDir := t.TempDir()
	h := NewHandler(service, HandlerOptions{Storage: images.NewStorage(storageDir)})
	body := &bytes.Buffer{}
	writer := multipart.NewWriter(body)
	part, err := writer.CreateFormFile("avatar", "avatar.png")
	if err != nil {
		t.Fatal(err)
	}
	_, _ = part.Write([]byte("\x89PNG\r\n\x1a\nexample-image"))
	_ = writer.Close()
	manager := middleware.NewTokenManager([]byte("01234567890123456789012345678901"), int64(time.Hour/time.Second))
	token, err := manager.Issue(9)
	if err != nil {
		t.Fatal(err)
	}
	req := httptest.NewRequest(http.MethodPatch, "/api/v1/users/me", body)
	req.Header.Set("Authorization", "Bearer "+token)
	req.Header.Set("Content-Type", writer.FormDataContentType())
	rec := httptest.NewRecorder()
	manager.Middleware(http.HandlerFunc(h.UpdateMe)).ServeHTTP(rec, req)
	if rec.Code != http.StatusInternalServerError {
		t.Fatalf("expected 500, got %d", rec.Code)
	}
	files, err := os.ReadDir(storageDir)
	if err != nil {
		t.Fatal(err)
	}
	if len(files) != 0 {
		t.Fatalf("orphan avatar was not removed: %v", files)
	}
}

func TestUpdateMeRejectsInvalidClearDescriptionFlag(t *testing.T) {
	service := &fakeUserService{user: &domain.User{ID: 9}}
	h := NewHandler(service)
	body := &bytes.Buffer{}
	writer := multipart.NewWriter(body)
	if err := writer.WriteField("clear_description", "sometimes"); err != nil {
		t.Fatal(err)
	}
	if err := writer.Close(); err != nil {
		t.Fatal(err)
	}
	manager := middleware.NewTokenManager([]byte("01234567890123456789012345678901"), int64(time.Hour/time.Second))
	token, err := manager.Issue(9)
	if err != nil {
		t.Fatal(err)
	}
	req := httptest.NewRequest(http.MethodPatch, "/api/v1/users/me", body)
	req.Header.Set("Authorization", "Bearer "+token)
	req.Header.Set("Content-Type", writer.FormDataContentType())
	rec := httptest.NewRecorder()
	manager.Middleware(http.HandlerFunc(h.UpdateMe)).ServeHTTP(rec, req)
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d: %s", rec.Code, rec.Body.String())
	}
}

func TestUpdateMeRejectsUnknownMultipartFields(t *testing.T) {
	service := &fakeUserService{user: &domain.User{ID: 9}}
	h := NewHandler(service)
	body := &bytes.Buffer{}
	writer := multipart.NewWriter(body)
	if err := writer.WriteField("avatar_url", "https://example.com/avatar.png"); err != nil {
		t.Fatal(err)
	}
	if err := writer.Close(); err != nil {
		t.Fatal(err)
	}
	manager := middleware.NewTokenManager([]byte("01234567890123456789012345678901"), int64(time.Hour/time.Second))
	token, err := manager.Issue(9)
	if err != nil {
		t.Fatal(err)
	}
	req := httptest.NewRequest(http.MethodPatch, "/api/v1/users/me", body)
	req.Header.Set("Authorization", "Bearer "+token)
	req.Header.Set("Content-Type", writer.FormDataContentType())
	rec := httptest.NewRecorder()
	manager.Middleware(http.HandlerFunc(h.UpdateMe)).ServeHTTP(rec, req)
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d: %s", rec.Code, rec.Body.String())
	}
}

func TestUpdateMeRejectsEmptyPatch(t *testing.T) {
	service := &fakeUserService{user: &domain.User{ID: 9, Name: "Alex", UserTag: "alex"}}
	h := NewHandler(service)
	rec := serveAsUser(t, 9, http.HandlerFunc(h.UpdateMe), http.MethodPatch, "/api/v1/users/me", `{}`)
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("expected 400 for empty update, got %d: %s", rec.Code, rec.Body.String())
	}
	if service.updatedID != 0 {
		t.Fatalf("service must not be called for an empty update, got user %d", service.updatedID)
	}
}
