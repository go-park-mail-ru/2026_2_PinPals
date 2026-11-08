package http

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

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

func TestUpdateMeCanClearNullableProfileFields(t *testing.T) {
	service := &fakeUserService{user: &domain.User{ID: 9, Name: "Alex", UserTag: "alex"}}
	h := NewHandler(service)
	rec := serveAsUser(t, 9, http.HandlerFunc(h.UpdateMe), http.MethodPatch, "/api/v1/users/me", `{"description":null,"avatar_url":null}`)
	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", rec.Code, rec.Body.String())
	}
	if service.updatedID != 9 || !service.updated.DescriptionSet || !service.updated.AvatarURLSet {
		t.Fatalf("nullable fields were not marked as explicitly set: %#v", service.updated)
	}
	if service.updated.Description != nil || service.updated.AvatarURL != nil {
		t.Fatalf("expected nullable fields to be cleared: %#v", service.updated)
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

type fakeRevoker struct {
	tokens []string
	err    error
}

func (f *fakeRevoker) Revoke(_ context.Context, token string) error {
	f.tokens = append(f.tokens, token)
	return f.err
}

func TestDeleteMeRevokesTokenAndClearsCookies(t *testing.T) {
	service := &fakeUserService{}
	revoker := &fakeRevoker{}
	h := NewHandler(service, HandlerOptions{TokenRevoker: revoker})
	rec := serveAsUser(t, 9, http.HandlerFunc(h.DeleteMe), http.MethodDelete, "/api/v1/users/me", "")
	if rec.Code != http.StatusNoContent || len(revoker.tokens) != 1 || revoker.tokens[0] == "" {
		t.Fatalf("expected 204 and one revoked token, got status=%d tokens=%v", rec.Code, revoker.tokens)
	}
	cleared := 0
	for _, c := range rec.Result().Cookies() {
		if (c.Name == middleware.AccessTokenCookieName || c.Name == middleware.CSRFTokenCookieName) && c.MaxAge < 0 {
			cleared++
		}
	}
	if cleared != 2 {
		t.Fatalf("expected both auth cookies to be cleared, got %d", cleared)
	}
}

func TestDeleteMeDoesNotRevokeWhenDeleteFails(t *testing.T) {
	service := &fakeUserService{deleteErr: errors.New("database unavailable")}
	revoker := &fakeRevoker{}
	h := NewHandler(service, HandlerOptions{TokenRevoker: revoker})
	rec := serveAsUser(t, 9, http.HandlerFunc(h.DeleteMe), http.MethodDelete, "/api/v1/users/me", "")
	if rec.Code != http.StatusInternalServerError || len(revoker.tokens) != 0 {
		t.Fatalf("expected 500 without revocation, got status=%d tokens=%v", rec.Code, revoker.tokens)
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
