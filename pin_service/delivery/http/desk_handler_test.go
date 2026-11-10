package http

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	httpmocks "2026_2_PinPals/pin_service/delivery/http/mocks"
	"2026_2_PinPals/pin_service/domain"
	"2026_2_PinPals/pkg/middleware"
	"go.uber.org/mock/gomock"
)

const testDeskJWTSecret = "01234567890123456789012345678901"

func serveDeskRequestAsUser(t *testing.T, handler http.Handler, method, target, body string, pathValues map[string]string) *httptest.ResponseRecorder {
	t.Helper()
	manager := middleware.NewTokenManager([]byte(testDeskJWTSecret), int64(time.Hour/time.Second))
	token, err := manager.Issue(17)
	if err != nil {
		t.Fatal(err)
	}
	req := httptest.NewRequest(method, target, strings.NewReader(body))
	if body != "" {
		req.Header.Set("Content-Type", "application/json")
	}
	req.AddCookie(&http.Cookie{Name: middleware.AccessTokenCookieName, Value: token})
	for key, value := range pathValues {
		req.SetPathValue(key, value)
	}
	rec := httptest.NewRecorder()
	middleware.Access(manager)(handler).ServeHTTP(rec, req)
	return rec
}

func TestDeskCreateValidatesJSONBeforeCallingService(t *testing.T) {
	cases := []struct {
		name string
		body string
	}{
		{name: "malformed JSON", body: "{"},
		{name: "unsupported status", body: `{"name":"Art","entry_status":9}`},
		{name: "overlong name", body: `{"name":"` + strings.Repeat("x", 256) + `","entry_status":0}`},
		{name: "overlong description", body: `{"name":"Art","entry_status":0,"description":"` + strings.Repeat("x", 2001) + `"}`},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			ctrl := gomock.NewController(t)
			service := httpmocks.NewMockDeskService(ctrl)
			handler := NewDeskHandler(service, "", nil)
			rec := serveDeskRequestAsUser(t, http.HandlerFunc(handler.Create), http.MethodPost, "/api/v1/desks", tc.body, nil)
			if rec.Code != http.StatusBadRequest {
				t.Fatalf("status=%d body=%s", rec.Code, rec.Body.String())
			}
		})
	}
}

func TestDeskUpdateValidatesPartialJSONAndMapsNull(t *testing.T) {
	t.Run("empty update rejected", func(t *testing.T) {
		ctrl := gomock.NewController(t)
		service := httpmocks.NewMockDeskService(ctrl)
		handler := NewDeskHandler(service, "", nil)
		rec := serveDeskRequestAsUser(t, http.HandlerFunc(handler.Update), http.MethodPatch, "/api/v1/desks/4", `{}`, map[string]string{"deskID": "4"})
		if rec.Code != http.StatusBadRequest {
			t.Fatalf("status=%d body=%s", rec.Code, rec.Body.String())
		}
	})

	t.Run("invalid nullable field rejected", func(t *testing.T) {
		ctrl := gomock.NewController(t)
		service := httpmocks.NewMockDeskService(ctrl)
		handler := NewDeskHandler(service, "", nil)
		rec := serveDeskRequestAsUser(t, http.HandlerFunc(handler.Update), http.MethodPatch, "/api/v1/desks/4", `{"description":123}`, map[string]string{"deskID": "4"})
		if rec.Code != http.StatusBadRequest {
			t.Fatalf("status=%d body=%s", rec.Code, rec.Body.String())
		}
	})

	t.Run("explicit null is passed as clear operation", func(t *testing.T) {
		ctrl := gomock.NewController(t)
		service := httpmocks.NewMockDeskService(ctrl)
		service.EXPECT().Update(gomock.Any(), 17, 4, domain.UpdateDeskInput{
			DescriptionSet: true,
		}).Return(domain.Desk{ID: 4, CreatorID: 17, Name: "Desk"}, nil)
		handler := NewDeskHandler(service, "", nil)
		rec := serveDeskRequestAsUser(t, http.HandlerFunc(handler.Update), http.MethodPatch, "/api/v1/desks/4", `{"description":null}`, map[string]string{"deskID": "4"})
		if rec.Code != http.StatusOK {
			t.Fatalf("status=%d body=%s", rec.Code, rec.Body.String())
		}
	})
}
