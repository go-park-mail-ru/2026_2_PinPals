package http

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"2026_2_PinPals/pin_service/domain"
	"2026_2_PinPals/pkg/middleware"
)

type fakeDeskHandlerService struct {
	createdUserID int
	created       domain.Desk
	getResult     domain.DeskWithPins
	getRequester  int
	getDeskID     int
	err           error
}

func (f *fakeDeskHandlerService) Create(_ context.Context, userID int, in domain.CreateDeskInput) (domain.Desk, error) {
	f.createdUserID = userID
	f.created = domain.Desk{ID: 1, CreatorID: userID, Name: in.Name, EntryStatus: in.EntryStatus}
	return f.created, f.err
}
func (f *fakeDeskHandlerService) ListOwn(context.Context, int) ([]domain.Desk, error) {
	return nil, f.err
}
func (f *fakeDeskHandlerService) ListPublic(context.Context, int, int) ([]domain.Desk, error) {
	return nil, f.err
}
func (f *fakeDeskHandlerService) GetWithPins(_ context.Context, userID, deskID int) (domain.DeskWithPins, error) {
	f.getRequester, f.getDeskID = userID, deskID
	return f.getResult, f.err
}
func (f *fakeDeskHandlerService) Update(context.Context, int, int, domain.UpdateDeskInput) (domain.Desk, error) {
	return domain.Desk{}, f.err
}
func (f *fakeDeskHandlerService) Delete(context.Context, int, int) error         { return f.err }
func (f *fakeDeskHandlerService) AddPin(context.Context, int, int, int) error    { return f.err }
func (f *fakeDeskHandlerService) RemovePin(context.Context, int, int, int) error { return f.err }

type fakeLikeHandlerService struct {
	userID int
	pinID  int
	stats  domain.LikeStats
	called string
	err    error
}

func (f *fakeLikeHandlerService) Add(_ context.Context, userID, pinID int) error {
	f.userID, f.pinID, f.called = userID, pinID, "add"
	return f.err
}
func (f *fakeLikeHandlerService) Remove(_ context.Context, userID, pinID int) error {
	f.userID, f.pinID, f.called = userID, pinID, "remove"
	return f.err
}
func (f *fakeLikeHandlerService) Stats(_ context.Context, userID, pinID int) (domain.LikeStats, error) {
	f.userID, f.pinID, f.called = userID, pinID, "stats"
	return f.stats, f.err
}

func servePinAsUser(t *testing.T, userID int, next http.Handler, method, target, body string) *httptest.ResponseRecorder {
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

func TestDeskCreateUsesAuthenticatedUser(t *testing.T) {
	service := &fakeDeskHandlerService{}
	h := NewDeskHandler(service, "")
	rec := servePinAsUser(t, 12, http.HandlerFunc(h.Create), http.MethodPost, "/api/v1/desks", `{"name":"Art","entry_status":0}`)
	if rec.Code != http.StatusCreated || service.createdUserID != 12 {
		t.Fatalf("status=%d creator=%d body=%s", rec.Code, service.createdUserID, rec.Body.String())
	}
}

func TestDeskGetByIDAllowsAnonymousPublicRead(t *testing.T) {
	service := &fakeDeskHandlerService{getResult: domain.DeskWithPins{Desk: domain.Desk{ID: 4, CreatorID: 12, EntryStatus: domain.EntryStatusPublic}}}
	h := NewDeskHandler(service, "")
	req := httptest.NewRequest(http.MethodGet, "/api/v1/desks/4", nil)
	req.SetPathValue("deskID", "4")
	rec := httptest.NewRecorder()
	middleware.OptionalAccess(middleware.NewTokenManager([]byte("01234567890123456789012345678901"), 3600))(http.HandlerFunc(h.GetByID)).ServeHTTP(rec, req)
	if rec.Code != http.StatusOK || service.getRequester != 0 || service.getDeskID != 4 {
		t.Fatalf("status=%d requester=%d desk=%d body=%s", rec.Code, service.getRequester, service.getDeskID, rec.Body.String())
	}
}

func TestLikeAddAndStatsUseAuthenticatedUser(t *testing.T) {
	service := &fakeLikeHandlerService{stats: domain.LikeStats{Count: 3, LikedByMe: true}}
	h := NewLikeHandler(service)
	mux := http.NewServeMux()
	mux.Handle("POST /api/v1/pins/{pinID}/like", http.HandlerFunc(h.Add))
	rec := servePinAsUser(t, 21, mux, http.MethodPost, "/api/v1/pins/8/like", "")
	if rec.Code != http.StatusOK || service.called != "stats" || service.userID != 21 || service.pinID != 8 {
		t.Fatalf("status=%d call=%s user=%d pin=%d body=%s", rec.Code, service.called, service.userID, service.pinID, rec.Body.String())
	}
}

func TestLikeStatsSupportsAnonymousRequest(t *testing.T) {
	service := &fakeLikeHandlerService{stats: domain.LikeStats{Count: 5}}
	h := NewLikeHandler(service)
	req := httptest.NewRequest(http.MethodGet, "/api/v1/pins/8/likes", nil)
	req.SetPathValue("pinID", "8")
	rec := httptest.NewRecorder()
	h.GetStats(rec, req)
	if rec.Code != http.StatusOK || service.userID != 0 || service.pinID != 8 {
		t.Fatalf("status=%d user=%d pin=%d", rec.Code, service.userID, service.pinID)
	}
}
