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

	"2026_2_PinPals/pin_service/domain"
	"2026_2_PinPals/pkg/middleware"
)

type coverageDeskService struct {
	calls       []string
	errByMethod map[string]error
	desk        domain.Desk
	desks       []domain.Desk
	withPins    domain.DeskWithPins
	userID      int
	ownerID     int
	deskID      int
	pinID       int
	limit       int
	createInput domain.CreateDeskInput
	updateInput domain.UpdateDeskInput
}

func (s *coverageDeskService) err(method string) error {
	return s.errByMethod[method]
}

func (s *coverageDeskService) record(method string) {
	s.calls = append(s.calls, method)
}

func (s *coverageDeskService) Create(_ context.Context, userID int, input domain.CreateDeskInput) (domain.Desk, error) {
	s.record("create")
	s.userID, s.createInput = userID, input
	if s.err("create") != nil {
		return domain.Desk{}, s.err("create")
	}
	if s.desk.ID == 0 {
		return domain.Desk{ID: 1, CreatorID: userID, Name: input.Name, EntryStatus: input.EntryStatus}, nil
	}
	return s.desk, nil
}

func (s *coverageDeskService) ListOwn(_ context.Context, userID int) ([]domain.Desk, error) {
	s.record("list-own")
	s.userID = userID
	return s.desks, s.err("list-own")
}

func (s *coverageDeskService) ListPublic(_ context.Context, ownerID, limit int) ([]domain.Desk, error) {
	s.record("list-public")
	s.ownerID, s.limit = ownerID, limit
	return s.desks, s.err("list-public")
}

func (s *coverageDeskService) GetWithPins(_ context.Context, userID, deskID int) (domain.DeskWithPins, error) {
	s.record("get")
	s.userID, s.deskID = userID, deskID
	return s.withPins, s.err("get")
}

func (s *coverageDeskService) Update(_ context.Context, userID, deskID int, input domain.UpdateDeskInput) (domain.Desk, error) {
	s.record("update")
	s.userID, s.deskID, s.updateInput = userID, deskID, input
	if s.err("update") != nil {
		return domain.Desk{}, s.err("update")
	}
	if s.desk.ID == 0 {
		return domain.Desk{ID: deskID, CreatorID: userID, Name: "updated"}, nil
	}
	return s.desk, nil
}

func (s *coverageDeskService) Delete(_ context.Context, userID, deskID int) error {
	s.record("delete")
	s.userID, s.deskID = userID, deskID
	return s.err("delete")
}

func (s *coverageDeskService) AddPin(_ context.Context, userID, deskID, pinID int) error {
	s.record("add-pin")
	s.userID, s.deskID, s.pinID = userID, deskID, pinID
	return s.err("add-pin")
}

func (s *coverageDeskService) RemovePin(_ context.Context, userID, deskID, pinID int) error {
	s.record("remove-pin")
	s.userID, s.deskID, s.pinID = userID, deskID, pinID
	return s.err("remove-pin")
}

type coverageLikeService struct {
	calls   []string
	errAdd  error
	errDel  error
	errStat error
	userID  int
	pinID   int
	stats   domain.LikeStats
}

func (s *coverageLikeService) Add(_ context.Context, userID, pinID int) error {
	s.calls = append(s.calls, "add")
	s.userID, s.pinID = userID, pinID
	return s.errAdd
}

func (s *coverageLikeService) Remove(_ context.Context, userID, pinID int) error {
	s.calls = append(s.calls, "remove")
	s.userID, s.pinID = userID, pinID
	return s.errDel
}

func (s *coverageLikeService) Stats(_ context.Context, userID, pinID int) (domain.LikeStats, error) {
	s.calls = append(s.calls, "stats")
	s.userID, s.pinID = userID, pinID
	return s.stats, s.errStat
}

func invokeWithUser(t *testing.T, userID int, handler http.Handler, method, target, body string, values map[string]string) *httptest.ResponseRecorder {
	t.Helper()
	req := httptest.NewRequest(method, target, strings.NewReader(body))
	if body != "" {
		req.Header.Set("Content-Type", "application/json")
	}
	for key, value := range values {
		req.SetPathValue(key, value)
	}
	rec := httptest.NewRecorder()
	if userID < 1 {
		handler.ServeHTTP(rec, req)
		return rec
	}
	manager := middleware.NewTokenManager([]byte("01234567890123456789012345678901"), int64(time.Hour/time.Second))
	token, err := manager.Issue(userID)
	if err != nil {
		t.Fatal(err)
	}
	req.Header.Set("Authorization", "Bearer "+token)
	manager.Middleware(handler).ServeHTTP(rec, req)
	return rec
}

func TestDeskHandlerListOwnBranches(t *testing.T) {
	t.Run("unauthorized", func(t *testing.T) {
		h := NewDeskHandler(&coverageDeskService{}, "")
		rec := invokeWithUser(t, 0, http.HandlerFunc(h.ListOwn), http.MethodGet, "/api/v1/desks/me", "", nil)
		if rec.Code != http.StatusUnauthorized {
			t.Fatalf("status=%d body=%s", rec.Code, rec.Body.String())
		}
	})

	t.Run("success", func(t *testing.T) {
		svc := &coverageDeskService{desks: []domain.Desk{{ID: 2, CreatorID: 7, Name: "Travel"}}}
		h := NewDeskHandler(svc, "")
		rec := invokeWithUser(t, 7, http.HandlerFunc(h.ListOwn), http.MethodGet, "/api/v1/desks/me", "", nil)
		if rec.Code != http.StatusOK || svc.userID != 7 || !strings.Contains(rec.Body.String(), "Travel") {
			t.Fatalf("status=%d user=%d body=%s", rec.Code, svc.userID, rec.Body.String())
		}
	})

	t.Run("service error", func(t *testing.T) {
		svc := &coverageDeskService{errByMethod: map[string]error{"list-own": errors.New("db down")}}
		h := NewDeskHandler(svc, "")
		rec := invokeWithUser(t, 7, http.HandlerFunc(h.ListOwn), http.MethodGet, "/api/v1/desks/me", "", nil)
		if rec.Code != http.StatusInternalServerError {
			t.Fatalf("status=%d body=%s", rec.Code, rec.Body.String())
		}
	})
}

func TestDeskHandlerListPublicBranches(t *testing.T) {
	h := NewDeskHandler(&coverageDeskService{}, "")
	bad := httptest.NewRequest(http.MethodGet, "/api/v1/users/nope/desks", nil)
	bad.SetPathValue("userID", "nope")
	badRec := httptest.NewRecorder()
	h.ListPublic(badRec, bad)
	if badRec.Code != http.StatusBadRequest {
		t.Fatalf("invalid path status=%d", badRec.Code)
	}

	svc := &coverageDeskService{desks: []domain.Desk{{ID: 4, CreatorID: 9, Name: "Public"}}}
	h = NewDeskHandler(svc, "")
	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/api/v1/users/9/desks", nil)
	req.SetPathValue("userID", "9")
	h.ListPublic(rec, req)
	if rec.Code != http.StatusOK || svc.ownerID != 9 || svc.limit != 100 || !strings.Contains(rec.Body.String(), "Public") {
		t.Fatalf("status=%d owner=%d limit=%d body=%s", rec.Code, svc.ownerID, svc.limit, rec.Body.String())
	}
}

func TestDeskHandlerCreateBadJSONAndMappedErrors(t *testing.T) {
	t.Run("bad json", func(t *testing.T) {
		h := NewDeskHandler(&coverageDeskService{}, "")
		rec := invokeWithUser(t, 3, http.HandlerFunc(h.Create), http.MethodPost, "/api/v1/desks", "{", nil)
		if rec.Code != http.StatusBadRequest {
			t.Fatalf("status=%d body=%s", rec.Code, rec.Body.String())
		}
	})

	cases := []struct {
		name string
		err  error
		want int
	}{
		{"validation", domain.ErrValidation, http.StatusBadRequest},
		{"not found", domain.ErrDeskNotFound, http.StatusNotFound},
		{"forbidden", domain.ErrDeskForbidden, http.StatusForbidden},
		{"pin not found", domain.ErrPinNotFound, http.StatusNotFound},
		{"internal", errors.New("storage failure"), http.StatusInternalServerError},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			svc := &coverageDeskService{errByMethod: map[string]error{"create": tc.err}}
			h := NewDeskHandler(svc, "")
			rec := invokeWithUser(t, 3, http.HandlerFunc(h.Create), http.MethodPost, "/api/v1/desks", `{"name":"Art","entry_status":0}`, nil)
			if rec.Code != tc.want {
				t.Fatalf("status=%d want=%d body=%s", rec.Code, tc.want, rec.Body.String())
			}
		})
	}
}

func TestDeskHandlerGetByIDBranches(t *testing.T) {
	h := NewDeskHandler(&coverageDeskService{}, "")
	bad := httptest.NewRequest(http.MethodGet, "/api/v1/desks/nope", nil)
	bad.SetPathValue("deskID", "nope")
	badRec := httptest.NewRecorder()
	h.GetByID(badRec, bad)
	if badRec.Code != http.StatusBadRequest {
		t.Fatalf("invalid id status=%d", badRec.Code)
	}

	pin := domain.Pin{ID: 8, CreatorID: 5, ImageURL: "pin.jpg", Name: "Beach"}
	svc := &coverageDeskService{withPins: domain.DeskWithPins{Desk: domain.Desk{ID: 4, CreatorID: 5, Name: "Summer", AvatarImgURL: stringPtr("avatar one.jpg")}, Pins: []domain.Pin{pin}}}
	h = NewDeskHandler(svc, "https://cdn.example")
	rec := invokeWithUser(t, 0, http.HandlerFunc(h.GetByID), http.MethodGet, "/api/v1/desks/4", "", map[string]string{"deskID": "4"})
	if rec.Code != http.StatusOK || svc.userID != 0 || svc.deskID != 4 || !strings.Contains(rec.Body.String(), "https://cdn.example/images/avatar%20one.jpg") || !strings.Contains(rec.Body.String(), "https://cdn.example/images/pin.jpg") {
		t.Fatalf("status=%d requester=%d desk=%d body=%s", rec.Code, svc.userID, svc.deskID, rec.Body.String())
	}

	for _, tc := range []struct {
		err  error
		want int
	}{{domain.ErrDeskForbidden, http.StatusForbidden}, {domain.ErrDeskNotFound, http.StatusNotFound}} {
		svc := &coverageDeskService{errByMethod: map[string]error{"get": tc.err}}
		h := NewDeskHandler(svc, "")
		req := httptest.NewRequest(http.MethodGet, "/api/v1/desks/4", nil)
		req.SetPathValue("deskID", "4")
		rec := httptest.NewRecorder()
		h.GetByID(rec, req)
		if rec.Code != tc.want {
			t.Fatalf("error=%v status=%d want=%d", tc.err, rec.Code, tc.want)
		}
	}
}

func TestDeskHandlerUpdateBranches(t *testing.T) {
	h := NewDeskHandler(&coverageDeskService{}, "")
	unauthorized := invokeWithUser(t, 0, http.HandlerFunc(h.Update), http.MethodPatch, "/api/v1/desks/4", `{}`, map[string]string{"deskID": "4"})
	if unauthorized.Code != http.StatusUnauthorized {
		t.Fatalf("unauthorized status=%d", unauthorized.Code)
	}
	invalidID := invokeWithUser(t, 5, http.HandlerFunc(h.Update), http.MethodPatch, "/api/v1/desks/nope", `{}`, map[string]string{"deskID": "nope"})
	if invalidID.Code != http.StatusBadRequest {
		t.Fatalf("invalid id status=%d", invalidID.Code)
	}
	badJSON := invokeWithUser(t, 5, http.HandlerFunc(h.Update), http.MethodPatch, "/api/v1/desks/4", "{", map[string]string{"deskID": "4"})
	if badJSON.Code != http.StatusBadRequest {
		t.Fatalf("bad json status=%d", badJSON.Code)
	}
	badField := invokeWithUser(t, 5, http.HandlerFunc(h.Update), http.MethodPatch, "/api/v1/desks/4", `{"description":{"nested":"no"}}`, map[string]string{"deskID": "4"})
	if badField.Code != http.StatusBadRequest {
		t.Fatalf("invalid nullable field status=%d", badField.Code)
	}

	svc := &coverageDeskService{}
	h = NewDeskHandler(svc, "")
	rec := invokeWithUser(t, 5, http.HandlerFunc(h.Update), http.MethodPatch, "/api/v1/desks/4", `{"description":null,"remove_avatar":true,"entry_status":1}`, map[string]string{"deskID": "4"})
	if rec.Code != http.StatusOK || svc.userID != 5 || svc.deskID != 4 || !svc.updateInput.DescriptionSet || svc.updateInput.Description != nil || !svc.updateInput.AvatarImgURLSet || svc.updateInput.AvatarImgURL != nil {
		t.Fatalf("status=%d input=%+v body=%s", rec.Code, svc.updateInput, rec.Body.String())
	}
}

func TestDeskHandlerDeleteAndChangePinBranches(t *testing.T) {
	svc := &coverageDeskService{}
	h := NewDeskHandler(svc, "")
	if rec := invokeWithUser(t, 0, http.HandlerFunc(h.Delete), http.MethodDelete, "/api/v1/desks/4", "", map[string]string{"deskID": "4"}); rec.Code != http.StatusUnauthorized {
		t.Fatalf("delete unauthorized status=%d", rec.Code)
	}
	if rec := invokeWithUser(t, 5, http.HandlerFunc(h.Delete), http.MethodDelete, "/api/v1/desks/nope", "", map[string]string{"deskID": "nope"}); rec.Code != http.StatusBadRequest {
		t.Fatalf("delete invalid id status=%d", rec.Code)
	}
	if rec := invokeWithUser(t, 5, http.HandlerFunc(h.Delete), http.MethodDelete, "/api/v1/desks/4", "", map[string]string{"deskID": "4"}); rec.Code != http.StatusNoContent || svc.deskID != 4 {
		t.Fatalf("delete status=%d desk=%d", rec.Code, svc.deskID)
	}

	if rec := invokeWithUser(t, 0, http.HandlerFunc(h.AddPin), http.MethodPost, "/api/v1/desks/4/pins/8", "", map[string]string{"deskID": "4", "pinID": "8"}); rec.Code != http.StatusUnauthorized {
		t.Fatalf("add pin unauthorized status=%d", rec.Code)
	}
	if rec := invokeWithUser(t, 5, http.HandlerFunc(h.AddPin), http.MethodPost, "/api/v1/desks/nope/pins/8", "", map[string]string{"deskID": "nope", "pinID": "8"}); rec.Code != http.StatusBadRequest {
		t.Fatalf("add pin invalid ids status=%d", rec.Code)
	}
	if rec := invokeWithUser(t, 5, http.HandlerFunc(h.AddPin), http.MethodPost, "/api/v1/desks/4/pins/8", "", map[string]string{"deskID": "4", "pinID": "8"}); rec.Code != http.StatusNoContent || svc.userID != 5 || svc.deskID != 4 || svc.pinID != 8 {
		t.Fatalf("add pin status=%d user=%d desk=%d pin=%d", rec.Code, svc.userID, svc.deskID, svc.pinID)
	}

	svc.errByMethod = map[string]error{"remove-pin": domain.ErrPinNotFound}
	if rec := invokeWithUser(t, 5, http.HandlerFunc(h.RemovePin), http.MethodDelete, "/api/v1/desks/4/pins/8", "", map[string]string{"deskID": "4", "pinID": "8"}); rec.Code != http.StatusNotFound {
		t.Fatalf("remove pin mapped error status=%d body=%s", rec.Code, rec.Body.String())
	}
}

func TestDeskDTOAndResponseCoverage(t *testing.T) {
	create := CreateDeskRequest{Name: "Art", EntryStatus: domain.EntryStatusPublic}
	if got := create.ToDomain(); got.Name != "Art" || got.EntryStatus != domain.EntryStatusPublic {
		t.Fatalf("unexpected create mapping: %+v", got)
	}

	var update UpdateDeskRequest
	if err := json.Unmarshal([]byte(`{"name":"New","description":"text","remove_avatar":true,"entry_status":1}`), &update); err != nil {
		t.Fatal(err)
	}
	input, err := update.ToDomain()
	if err != nil || input.Name == nil || *input.Name != "New" || input.Description == nil || *input.Description != "text" || !input.DescriptionSet || input.AvatarImgURL != nil || !input.AvatarImgURLSet {
		t.Fatalf("input=%+v err=%v", input, err)
	}
	if _, err := (UpdateDeskRequest{Description: json.RawMessage(`123`)}).ToDomain(); err == nil {
		t.Fatal("expected non-string nullable value to fail")
	}
	if value, err := decodeDeskNullableString(json.RawMessage(`"ok"`)); err != nil || value == nil || *value != "ok" {
		t.Fatalf("value=%v err=%v", value, err)
	}
	if value, err := decodeDeskNullableString(json.RawMessage(`null`)); err != nil || value != nil {
		t.Fatalf("null value=%v err=%v", value, err)
	}

	desk := domain.Desk{ID: 3, CreatorID: 5, Name: "Art", AvatarImgURL: stringPtr("cover.jpg")}
	pins := []domain.Pin{{ID: 8, ImageURL: "pin.jpg", Name: "Beach"}}
	resp := NewDeskHandler(&coverageDeskService{}, "https://cdn.example").response(desk, pins)
	if resp.AvatarImgURL == nil || *resp.AvatarImgURL != "https://cdn.example/images/cover.jpg" || len(resp.Pins) != 1 || resp.Pins[0].ImageURL != "https://cdn.example/images/pin.jpg" {
		t.Fatalf("unexpected response mapping: %+v", resp)
	}
	withoutURL := NewDeskHandler(&coverageDeskService{}, "").response(desk, nil)
	if withoutURL.AvatarImgURL == nil || *withoutURL.AvatarImgURL != "/images/cover.jpg" || withoutURL.Pins != nil {
		t.Fatalf("unexpected no-base-url response: %+v", withoutURL)
	}
}

func TestLikeHandlerAddAllBranches(t *testing.T) {
	h := NewLikeHandler(&coverageLikeService{})
	if rec := invokeWithUser(t, 0, http.HandlerFunc(h.Add), http.MethodPost, "/api/v1/pins/8/like", "", map[string]string{"pinID": "8"}); rec.Code != http.StatusUnauthorized {
		t.Fatalf("unauthorized status=%d", rec.Code)
	}
	if rec := invokeWithUser(t, 9, http.HandlerFunc(h.Add), http.MethodPost, "/api/v1/pins/nope/like", "", map[string]string{"pinID": "nope"}); rec.Code != http.StatusBadRequest {
		t.Fatalf("invalid id status=%d", rec.Code)
	}

	svc := &coverageLikeService{stats: domain.LikeStats{Count: 2, LikedByMe: true}}
	h = NewLikeHandler(svc)
	rec := invokeWithUser(t, 9, http.HandlerFunc(h.Add), http.MethodPost, "/api/v1/pins/8/like", "", map[string]string{"pinID": "8"})
	if rec.Code != http.StatusOK || svc.userID != 9 || svc.pinID != 8 || strings.Join(svc.calls, ",") != "add,stats" {
		t.Fatalf("status=%d calls=%v user=%d pin=%d body=%s", rec.Code, svc.calls, svc.userID, svc.pinID, rec.Body.String())
	}

	for _, tc := range []struct {
		err  error
		want int
	}{{domain.ErrValidation, http.StatusBadRequest}, {domain.ErrPinNotFound, http.StatusNotFound}, {errors.New("db down"), http.StatusInternalServerError}} {
		svc := &coverageLikeService{errAdd: tc.err}
		h := NewLikeHandler(svc)
		rec := invokeWithUser(t, 9, http.HandlerFunc(h.Add), http.MethodPost, "/api/v1/pins/8/like", "", map[string]string{"pinID": "8"})
		if rec.Code != tc.want {
			t.Fatalf("error=%v status=%d want=%d", tc.err, rec.Code, tc.want)
		}
	}

	svc = &coverageLikeService{errStat: errors.New("stats failed")}
	h = NewLikeHandler(svc)
	rec = invokeWithUser(t, 9, http.HandlerFunc(h.Add), http.MethodPost, "/api/v1/pins/8/like", "", map[string]string{"pinID": "8"})
	if rec.Code != http.StatusInternalServerError {
		t.Fatalf("stats error status=%d", rec.Code)
	}
}

func TestLikeHandlerRemoveAllBranches(t *testing.T) {
	h := NewLikeHandler(&coverageLikeService{})
	if rec := invokeWithUser(t, 0, http.HandlerFunc(h.Remove), http.MethodDelete, "/api/v1/pins/8/like", "", map[string]string{"pinID": "8"}); rec.Code != http.StatusUnauthorized {
		t.Fatalf("unauthorized status=%d", rec.Code)
	}
	if rec := invokeWithUser(t, 4, http.HandlerFunc(h.Remove), http.MethodDelete, "/api/v1/pins/nope/like", "", map[string]string{"pinID": "nope"}); rec.Code != http.StatusBadRequest {
		t.Fatalf("invalid id status=%d", rec.Code)
	}
	svc := &coverageLikeService{stats: domain.LikeStats{Count: 1}}
	h = NewLikeHandler(svc)
	rec := invokeWithUser(t, 4, http.HandlerFunc(h.Remove), http.MethodDelete, "/api/v1/pins/8/like", "", map[string]string{"pinID": "8"})
	if rec.Code != http.StatusOK || strings.Join(svc.calls, ",") != "remove,stats" {
		t.Fatalf("status=%d calls=%v body=%s", rec.Code, svc.calls, rec.Body.String())
	}
	for _, tc := range []struct {
		err  error
		want int
	}{{domain.ErrValidation, http.StatusBadRequest}, {domain.ErrPinNotFound, http.StatusNotFound}, {errors.New("db down"), http.StatusInternalServerError}} {
		svc := &coverageLikeService{errDel: tc.err}
		h := NewLikeHandler(svc)
		rec := invokeWithUser(t, 4, http.HandlerFunc(h.Remove), http.MethodDelete, "/api/v1/pins/8/like", "", map[string]string{"pinID": "8"})
		if rec.Code != tc.want {
			t.Fatalf("error=%v status=%d want=%d", tc.err, rec.Code, tc.want)
		}
	}
	svc = &coverageLikeService{errStat: errors.New("stats failed")}
	h = NewLikeHandler(svc)
	rec = invokeWithUser(t, 4, http.HandlerFunc(h.Remove), http.MethodDelete, "/api/v1/pins/8/like", "", map[string]string{"pinID": "8"})
	if rec.Code != http.StatusInternalServerError {
		t.Fatalf("stats error status=%d", rec.Code)
	}
}

func TestLikeHandlerGetStatsBranches(t *testing.T) {
	h := NewLikeHandler(&coverageLikeService{})
	bad := httptest.NewRequest(http.MethodGet, "/api/v1/pins/nope/likes", nil)
	bad.SetPathValue("pinID", "nope")
	badRec := httptest.NewRecorder()
	h.GetStats(badRec, bad)
	if badRec.Code != http.StatusBadRequest {
		t.Fatalf("invalid id status=%d", badRec.Code)
	}

	svc := &coverageLikeService{stats: domain.LikeStats{Count: 7}}
	h = NewLikeHandler(svc)
	rec := invokeWithUser(t, 0, http.HandlerFunc(h.GetStats), http.MethodGet, "/api/v1/pins/8/likes", "", map[string]string{"pinID": "8"})
	if rec.Code != http.StatusOK || svc.userID != 0 || svc.pinID != 8 {
		t.Fatalf("status=%d user=%d pin=%d body=%s", rec.Code, svc.userID, svc.pinID, rec.Body.String())
	}

	svc = &coverageLikeService{errStat: domain.ErrPinNotFound}
	h = NewLikeHandler(svc)
	rec = invokeWithUser(t, 0, http.HandlerFunc(h.GetStats), http.MethodGet, "/api/v1/pins/8/likes", "", map[string]string{"pinID": "8"})
	if rec.Code != http.StatusNotFound {
		t.Fatalf("not found status=%d", rec.Code)
	}
}

func stringPtr(s string) *string { return &s }
