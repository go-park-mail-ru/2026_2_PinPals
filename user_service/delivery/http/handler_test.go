package http

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"2026_2_PinPals/pkg/images"
	"2026_2_PinPals/pkg/middleware"
	httpmocks "2026_2_PinPals/user_service/delivery/http/mocks"
	"2026_2_PinPals/user_service/domain"
	"go.uber.org/mock/gomock"
)

func serveAsUser(t *testing.T, userID int, next http.Handler, method, target string, body string) *httptest.ResponseRecorder {
	t.Helper()
	req := httptest.NewRequest(method, target, bytes.NewBufferString(body))
	if body != "" {
		req.Header.Set("Content-Type", "application/json")
	}
	return serveAsUserRequest(t, userID, next, req)
}

func serveAsUserRequest(t *testing.T, userID int, next http.Handler, req *http.Request) *httptest.ResponseRecorder {
	t.Helper()
	manager := middleware.NewTokenManager([]byte("01234567890123456789012345678901"), int64(time.Hour/time.Second))
	token, err := manager.Issue(userID)
	if err != nil {
		t.Fatal(err)
	}
	req.Header.Set("Authorization", "Bearer "+token)
	rec := httptest.NewRecorder()
	manager.Middleware(next).ServeHTTP(rec, req)
	return rec
}

func TestGetMeReturnsCurrentUser(t *testing.T) {
	ctrl := gomock.NewController(t)
	service := httpmocks.NewMockUserService(ctrl)
	service.EXPECT().GetProfile(gomock.Any(), 9).Return(&domain.User{ID: 9, Name: "Alex", UserTag: "alex"}, nil)
	h := NewHandler(service)
	rec := serveAsUser(t, 9, http.HandlerFunc(h.GetMe), http.MethodGet, "/api/v1/users/me", "")
	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", rec.Code, rec.Body.String())
	}
	var got UserResponse
	if err := json.Unmarshal(rec.Body.Bytes(), &got); err != nil {
		t.Fatal(err)
	}
	if got.ID != 9 {
		t.Fatalf("unexpected user response: %#v", got)
	}
}

func TestUpdateMeCanClearNullableDescription(t *testing.T) {
	ctrl := gomock.NewController(t)
	service := httpmocks.NewMockUserService(ctrl)
	service.EXPECT().UpdateProfile(gomock.Any(), 9, domain.UpdateProfileInput{DescriptionSet: true}).
		Return(&domain.User{ID: 9, Name: "Alex", UserTag: "alex"}, nil)
	h := NewHandler(service)
	rec := serveAsUser(t, 9, http.HandlerFunc(h.UpdateMe), http.MethodPatch, "/api/v1/users/me", `{"description":null}`)
	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", rec.Code, rec.Body.String())
	}
}

func TestUpdateMeRejectsClientSuppliedAvatarURL(t *testing.T) {
	ctrl := gomock.NewController(t)
	service := httpmocks.NewMockUserService(ctrl)
	h := NewHandler(service)
	rec := serveAsUser(t, 9, http.HandlerFunc(h.UpdateMe), http.MethodPatch, "/api/v1/users/me", `{"avatar_url":"https://example.test/avatar.png"}`)
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d: %s", rec.Code, rec.Body.String())
	}
}

func TestUpdateMeRejectsInvalidNullableValue(t *testing.T) {
	ctrl := gomock.NewController(t)
	service := httpmocks.NewMockUserService(ctrl)
	h := NewHandler(service)
	rec := serveAsUser(t, 9, http.HandlerFunc(h.UpdateMe), http.MethodPatch, "/api/v1/users/me", `{"description":42}`)
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d", rec.Code)
	}
}

func TestUpdateMeUploadsAvatarAsMultipartImage(t *testing.T) {
	ctrl := gomock.NewController(t)
	service := httpmocks.NewMockUserService(ctrl)
	storage := images.NewStorage(t.TempDir())
	service.EXPECT().GetProfile(gomock.Any(), 9).Return(&domain.User{ID: 9, Name: "Alex"}, nil)
	service.EXPECT().UpdateProfile(gomock.Any(), 9, gomock.Any()).DoAndReturn(
		func(_ context.Context, _ int, input domain.UpdateProfileInput) (*domain.User, error) {
			if !input.AvatarURLSet || input.AvatarURL == nil || !images.IsStoredFilename(*input.AvatarURL) {
				t.Fatalf("expected stored avatar filename, got %#v", input)
			}
			return &domain.User{ID: 9, Name: "Alex", AvatarURL: input.AvatarURL}, nil
		},
	)
	var body bytes.Buffer
	writer := multipart.NewWriter(&body)
	part, err := writer.CreateFormFile("avatar", "avatar.png")
	if err != nil {
		t.Fatal(err)
	}
	_, _ = part.Write([]byte("\x89PNG\r\n\x1a\nimage-data"))
	_ = writer.WriteField("name", "Alex")
	if err := writer.Close(); err != nil {
		t.Fatal(err)
	}
	req := httptest.NewRequest(http.MethodPatch, "/api/v1/users/me", &body)
	req.Header.Set("Content-Type", writer.FormDataContentType())
	h := NewHandler(service, HandlerOptions{BaseURL: "https://example.test", ImageStorage: storage})
	rec := serveAsUserRequest(t, 9, http.HandlerFunc(h.UpdateMe), req)
	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", rec.Code, rec.Body.String())
	}
}

func TestDeleteMeCallsService(t *testing.T) {
	ctrl := gomock.NewController(t)
	service := httpmocks.NewMockUserService(ctrl)
	service.EXPECT().DeleteProfile(gomock.Any(), 9).Return(nil)
	h := NewHandler(service)
	rec := serveAsUser(t, 9, http.HandlerFunc(h.DeleteMe), http.MethodDelete, "/api/v1/users/me", "")
	if rec.Code != http.StatusNoContent {
		t.Fatalf("unexpected delete status: %d", rec.Code)
	}
}

func TestDeleteMeRevokesTokenAndClearsCookies(t *testing.T) {
	ctrl := gomock.NewController(t)
	service := httpmocks.NewMockUserService(ctrl)
	revoker := httpmocks.NewMockTokenRevoker(ctrl)
	service.EXPECT().DeleteProfile(gomock.Any(), 9).Return(nil)
	revoker.EXPECT().Revoke(gomock.Any(), gomock.Any()).DoAndReturn(func(_ context.Context, token string) error {
		if token == "" {
			t.Fatal("revoked token must not be empty")
		}
		return nil
	})
	h := NewHandler(service, HandlerOptions{TokenRevoker: revoker})
	rec := serveAsUser(t, 9, http.HandlerFunc(h.DeleteMe), http.MethodDelete, "/api/v1/users/me", "")
	if rec.Code != http.StatusNoContent {
		t.Fatalf("expected 204, got %d", rec.Code)
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
	ctrl := gomock.NewController(t)
	service := httpmocks.NewMockUserService(ctrl)
	revoker := httpmocks.NewMockTokenRevoker(ctrl)
	service.EXPECT().DeleteProfile(gomock.Any(), 9).Return(errors.New("database unavailable"))
	h := NewHandler(service, HandlerOptions{TokenRevoker: revoker})
	rec := serveAsUser(t, 9, http.HandlerFunc(h.DeleteMe), http.MethodDelete, "/api/v1/users/me", "")
	if rec.Code != http.StatusInternalServerError {
		t.Fatalf("expected 500, got %d", rec.Code)
	}
}

func TestGetMeMapsNotFound(t *testing.T) {
	ctrl := gomock.NewController(t)
	service := httpmocks.NewMockUserService(ctrl)
	service.EXPECT().GetProfile(gomock.Any(), 9).Return(nil, domain.ErrUserNotFound)
	h := NewHandler(service)
	rec := serveAsUser(t, 9, http.HandlerFunc(h.GetMe), http.MethodGet, "/api/v1/users/me", "")
	if rec.Code != http.StatusNotFound {
		t.Fatalf("expected 404, got %d", rec.Code)
	}
}

func TestDeleteMeMapsInternalError(t *testing.T) {
	ctrl := gomock.NewController(t)
	service := httpmocks.NewMockUserService(ctrl)
	service.EXPECT().DeleteProfile(gomock.Any(), 9).Return(errors.New("database unavailable"))
	h := NewHandler(service)
	rec := serveAsUser(t, 9, http.HandlerFunc(h.DeleteMe), http.MethodDelete, "/api/v1/users/me", "")
	if rec.Code != http.StatusInternalServerError {
		t.Fatalf("expected 500, got %d", rec.Code)
	}
}
