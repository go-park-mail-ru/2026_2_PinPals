package http

import (
	"bytes"
	"context"
	"encoding/json"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	httpmocks "2026_2_PinPals/pin_service/delivery/http/mocks"
	"2026_2_PinPals/pin_service/domain"
	"2026_2_PinPals/pkg/images"
	"2026_2_PinPals/pkg/middleware"
	"go.uber.org/mock/gomock"
)

const deskTestJWTSecret = "01234567890123456789012345678901"

func serveDeskAsUser(t *testing.T, handler http.Handler, req *http.Request) *httptest.ResponseRecorder {
	t.Helper()
	manager := middleware.NewTokenManager([]byte(deskTestJWTSecret), int64(time.Hour/time.Second))
	token, err := manager.Issue(17)
	if err != nil {
		t.Fatal(err)
	}
	req.AddCookie(&http.Cookie{Name: middleware.AccessTokenCookieName, Value: token})
	rec := httptest.NewRecorder()
	middleware.Access(manager)(handler).ServeHTTP(rec, req)
	return rec
}

func TestDeskCreateRejectsURLAvatarAndInvalidData(t *testing.T) {
	for _, body := range []string{
		`{"avatar_img_url":"https://example.test/avatar.png"}`,
		`{"name":"` + strings.Repeat("x", 256) + `"}`,
		`{"entry_status":9}`,
		`{"description":"` + strings.Repeat("x", 2001) + `"}`,
	} {
		ctrl := gomock.NewController(t)
		service := httpmocks.NewMockDeskService(ctrl)
		h := NewDeskHandler(service, "")
		req := httptest.NewRequest(http.MethodPost, "/api/v1/desks", strings.NewReader(body))
		req.Header.Set("Content-Type", "application/json")
		rec := serveDeskAsUser(t, http.HandlerFunc(h.Create), req)
		if rec.Code != http.StatusBadRequest {
			t.Fatalf("body %q: expected 400, got %d: %s", body, rec.Code, rec.Body.String())
		}
	}
}

func TestDeskCreateUploadsAvatarFile(t *testing.T) {
	ctrl := gomock.NewController(t)
	service := httpmocks.NewMockDeskService(ctrl)
	storage := images.NewStorage(t.TempDir())
	service.EXPECT().Create(gomock.Any(), 17, gomock.Any()).DoAndReturn(
		func(_ context.Context, _ int, input domain.CreateDeskInput) (domain.Desk, error) {
			if input.AvatarImgURL == nil || !images.IsStoredFilename(*input.AvatarImgURL) {
				t.Fatalf("expected uploaded image filename, got %#v", input.AvatarImgURL)
			}
			return domain.Desk{ID: 4, CreatorID: 17, Name: input.Name, AvatarImgURL: input.AvatarImgURL}, nil
		},
	)
	var body bytes.Buffer
	writer := multipart.NewWriter(&body)
	_ = writer.WriteField("name", "Art")
	_ = writer.WriteField("entry_status", "0")
	part, err := writer.CreateFormFile("avatar", "cover.png")
	if err != nil {
		t.Fatal(err)
	}
	_, _ = part.Write([]byte("\x89PNG\r\n\x1a\nimage-data"))
	if err := writer.Close(); err != nil {
		t.Fatal(err)
	}
	req := httptest.NewRequest(http.MethodPost, "/api/v1/desks", &body)
	req.Header.Set("Content-Type", writer.FormDataContentType())
	h := NewDeskHandler(service, "https://example.test", storage)
	rec := serveDeskAsUser(t, http.HandlerFunc(h.Create), req)
	if rec.Code != http.StatusCreated {
		t.Fatalf("expected 201, got %d: %s", rec.Code, rec.Body.String())
	}
	var got DeskResponse
	if err := json.Unmarshal(rec.Body.Bytes(), &got); err != nil {
		t.Fatal(err)
	}
	if got.AvatarImgURL == nil || !strings.HasPrefix(*got.AvatarImgURL, "https://example.test/images/") {
		t.Fatalf("expected public avatar URL, got %#v", got.AvatarImgURL)
	}
}

func TestDeskUpdateRejectsClientURL(t *testing.T) {
	ctrl := gomock.NewController(t)
	service := httpmocks.NewMockDeskService(ctrl)
	h := NewDeskHandler(service, "")
	req := httptest.NewRequest(http.MethodPatch, "/api/v1/desks/4", strings.NewReader(`{"avatar_img_url":"https://example.test/a.png"}`))
	req.Header.Set("Content-Type", "application/json")
	req.SetPathValue("deskID", "4")
	rec := serveDeskAsUser(t, http.HandlerFunc(h.Update), req)
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d", rec.Code)
	}
}

func TestDeskUpdateCanRemoveStoredAvatar(t *testing.T) {
	ctrl := gomock.NewController(t)
	service := httpmocks.NewMockDeskService(ctrl)
	imageDir := t.TempDir()
	storage := images.NewStorage(imageDir)
	oldName, err := storage.Save(bytes.NewReader([]byte("\x89PNG\r\n\x1a\nold-image")))
	if err != nil {
		t.Fatal(err)
	}
	service.EXPECT().GetWithPins(gomock.Any(), 17, 4).Return(domain.DeskWithPins{
		Desk: domain.Desk{ID: 4, CreatorID: 17, Name: "Art", AvatarImgURL: &oldName},
	}, nil)
	service.EXPECT().Update(gomock.Any(), 17, 4, domain.UpdateDeskInput{AvatarImgURLSet: true}).Return(
		domain.Desk{ID: 4, CreatorID: 17, Name: "Art"}, nil,
	)
	h := NewDeskHandler(service, "", storage)
	req := httptest.NewRequest(http.MethodPatch, "/api/v1/desks/4", strings.NewReader(`{"remove_avatar":true}`))
	req.Header.Set("Content-Type", "application/json")
	req.SetPathValue("deskID", "4")
	rec := serveDeskAsUser(t, http.HandlerFunc(h.Update), req)
	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", rec.Code, rec.Body.String())
	}
	if _, err := os.Stat(filepath.Join(imageDir, oldName)); err == nil {
		t.Fatal("old avatar should be removed")
	}
}
