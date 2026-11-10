package http

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	httpmocks "2026_2_PinPals/pin_service/delivery/http/mocks"
	"2026_2_PinPals/pin_service/domain"
	"2026_2_PinPals/pkg/middleware"
	"go.uber.org/mock/gomock"
)

func TestDeskCreateUsesAuthenticatedUser(t *testing.T) {
	ctrl := gomock.NewController(t)
	service := httpmocks.NewMockDeskService(ctrl)
	service.EXPECT().Create(gomock.Any(), 17, domain.CreateDeskInput{
		Name: "Art", EntryStatus: domain.EntryStatusPublic,
	}).Return(domain.Desk{ID: 1, CreatorID: 17, Name: "Art"}, nil)
	h := NewDeskHandler(service, "")
	req := httptest.NewRequest(http.MethodPost, "/api/v1/desks", strings.NewReader(`{"name":"Art","entry_status":0}`))
	req.Header.Set("Content-Type", "application/json")
	rec := serveDeskAsUser(t, http.HandlerFunc(h.Create), req)
	if rec.Code != http.StatusCreated {
		t.Fatalf("status=%d body=%s", rec.Code, rec.Body.String())
	}
}

func TestDeskGetByIDAllowsAnonymousPublicRead(t *testing.T) {
	ctrl := gomock.NewController(t)
	service := httpmocks.NewMockDeskService(ctrl)
	service.EXPECT().GetWithPins(gomock.Any(), 0, 4).Return(domain.DeskWithPins{
		Desk: domain.Desk{ID: 4, CreatorID: 12, EntryStatus: domain.EntryStatusPublic},
	}, nil)
	h := NewDeskHandler(service, "")
	req := httptest.NewRequest(http.MethodGet, "/api/v1/desks/4", nil)
	req.SetPathValue("deskID", "4")
	rec := httptest.NewRecorder()
	middleware.OptionalAccess(middleware.NewTokenManager([]byte("01234567890123456789012345678901"), 3600))(http.HandlerFunc(h.GetByID)).ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("status=%d body=%s", rec.Code, rec.Body.String())
	}
}

func TestLikeAddAndStatsUseAuthenticatedUser(t *testing.T) {
	ctrl := gomock.NewController(t)
	service := httpmocks.NewMockLikeService(ctrl)
	service.EXPECT().Add(gomock.Any(), 17, 8).Return(nil)
	service.EXPECT().Stats(gomock.Any(), 17, 8).Return(domain.LikeStats{Count: 3, LikedByMe: true}, nil)
	h := NewLikeHandler(service)
	mux := http.NewServeMux()
	mux.Handle("POST /api/v1/pins/{pinID}/like", http.HandlerFunc(h.Add))
	req := httptest.NewRequest(http.MethodPost, "/api/v1/pins/8/like", nil)
	rec := serveDeskAsUser(t, mux, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("status=%d body=%s", rec.Code, rec.Body.String())
	}
}

func TestLikeStatsSupportsAnonymousRequest(t *testing.T) {
	ctrl := gomock.NewController(t)
	service := httpmocks.NewMockLikeService(ctrl)
	service.EXPECT().Stats(gomock.Any(), 0, 8).Return(domain.LikeStats{Count: 5}, nil)
	h := NewLikeHandler(service)
	req := httptest.NewRequest(http.MethodGet, "/api/v1/pins/8/likes", nil)
	req.SetPathValue("pinID", "8")
	rec := httptest.NewRecorder()
	h.GetStats(rec, req)
	if rec.Code != http.StatusOK || !strings.Contains(rec.Body.String(), `"likes_count":5`) {
		t.Fatalf("status=%d body=%s", rec.Code, rec.Body.String())
	}
}
