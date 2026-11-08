package http

import (
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"

	"2026_2_PinPals/pin_service/domain"
)

func TestDeskHandlerRemainingServiceErrorPaths(t *testing.T) {
	t.Run("public list service error", func(t *testing.T) {
		svc := &coverageDeskService{errByMethod: map[string]error{"list-public": errors.New("query failed")}}
		h := NewDeskHandler(svc, "")
		req := httptest.NewRequest(http.MethodGet, "/api/v1/users/9/desks", nil)
		req.SetPathValue("userID", "9")
		rec := httptest.NewRecorder()
		h.ListPublic(rec, req)
		if rec.Code != http.StatusInternalServerError {
			t.Fatalf("status=%d body=%s", rec.Code, rec.Body.String())
		}
	})

	t.Run("get desk internal error", func(t *testing.T) {
		svc := &coverageDeskService{errByMethod: map[string]error{"get": errors.New("query failed")}}
		h := NewDeskHandler(svc, "")
		req := httptest.NewRequest(http.MethodGet, "/api/v1/desks/4", nil)
		req.SetPathValue("deskID", "4")
		rec := httptest.NewRecorder()
		h.GetByID(rec, req)
		if rec.Code != http.StatusInternalServerError {
			t.Fatalf("status=%d body=%s", rec.Code, rec.Body.String())
		}
	})

	t.Run("update maps all service error classes", func(t *testing.T) {
		cases := []struct {
			name string
			err  error
			want int
		}{
			{name: "validation", err: domain.ErrValidation, want: http.StatusBadRequest},
			{name: "not found", err: domain.ErrDeskNotFound, want: http.StatusNotFound},
			{name: "forbidden", err: domain.ErrDeskForbidden, want: http.StatusForbidden},
			{name: "pin not found", err: domain.ErrPinNotFound, want: http.StatusNotFound},
			{name: "internal", err: errors.New("database offline"), want: http.StatusInternalServerError},
		}
		for _, tc := range cases {
			t.Run(tc.name, func(t *testing.T) {
				svc := &coverageDeskService{errByMethod: map[string]error{"update": tc.err}}
				h := NewDeskHandler(svc, "")
				rec := invokeWithUser(t, 7, http.HandlerFunc(h.Update), http.MethodPatch, "/api/v1/desks/4", `{"name":"Updated"}`, map[string]string{"deskID": "4"})
				if rec.Code != tc.want {
					t.Fatalf("status=%d want=%d body=%s", rec.Code, tc.want, rec.Body.String())
				}
			})
		}
	})

	t.Run("delete maps service errors", func(t *testing.T) {
		cases := []struct {
			name string
			err  error
			want int
		}{
			{name: "not found", err: domain.ErrDeskNotFound, want: http.StatusNotFound},
			{name: "forbidden", err: domain.ErrDeskForbidden, want: http.StatusForbidden},
			{name: "internal", err: errors.New("database offline"), want: http.StatusInternalServerError},
		}
		for _, tc := range cases {
			t.Run(tc.name, func(t *testing.T) {
				svc := &coverageDeskService{errByMethod: map[string]error{"delete": tc.err}}
				h := NewDeskHandler(svc, "")
				rec := invokeWithUser(t, 7, http.HandlerFunc(h.Delete), http.MethodDelete, "/api/v1/desks/4", "", map[string]string{"deskID": "4"})
				if rec.Code != tc.want {
					t.Fatalf("status=%d want=%d body=%s", rec.Code, tc.want, rec.Body.String())
				}
			})
		}
	})

	t.Run("add and remove pin map service errors and success", func(t *testing.T) {
		cases := []struct {
			name   string
			method string
			path   string
			call   func(*DeskHandler, http.ResponseWriter, *http.Request)
			err    error
			want   int
		}{
			{name: "add forbidden", method: http.MethodPost, path: "/api/v1/desks/4/pins/8", call: (*DeskHandler).AddPin, err: domain.ErrDeskForbidden, want: http.StatusForbidden},
			{name: "add internal", method: http.MethodPost, path: "/api/v1/desks/4/pins/8", call: (*DeskHandler).AddPin, err: errors.New("database offline"), want: http.StatusInternalServerError},
			{name: "remove desk missing", method: http.MethodDelete, path: "/api/v1/desks/4/pins/8", call: (*DeskHandler).RemovePin, err: domain.ErrDeskNotFound, want: http.StatusNotFound},
			{name: "remove internal", method: http.MethodDelete, path: "/api/v1/desks/4/pins/8", call: (*DeskHandler).RemovePin, err: errors.New("database offline"), want: http.StatusInternalServerError},
		}
		for _, tc := range cases {
			t.Run(tc.name, func(t *testing.T) {
				methodKey := "add-pin"
				if tc.method == http.MethodDelete {
					methodKey = "remove-pin"
				}
				svc := &coverageDeskService{errByMethod: map[string]error{methodKey: tc.err}}
				h := NewDeskHandler(svc, "")
				rec := invokeWithUser(t, 7, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { tc.call(h, w, r) }), tc.method, tc.path, "", map[string]string{"deskID": "4", "pinID": "8"})
				if rec.Code != tc.want {
					t.Fatalf("status=%d want=%d body=%s", rec.Code, tc.want, rec.Body.String())
				}
			})
		}

		svc := &coverageDeskService{}
		h := NewDeskHandler(svc, "")
		rec := invokeWithUser(t, 7, http.HandlerFunc(h.RemovePin), http.MethodDelete, "/api/v1/desks/4/pins/8", "", map[string]string{"deskID": "4", "pinID": "8"})
		if rec.Code != http.StatusNoContent || len(svc.calls) == 0 || svc.calls[len(svc.calls)-1] != "remove-pin" {
			t.Fatalf("status=%d calls=%v body=%s", rec.Code, svc.calls, rec.Body.String())
		}
	})
}

func TestLikeGetStatsMapsInternalErrors(t *testing.T) {
	svc := &coverageLikeService{errStat: errors.New("database unavailable")}
	h := NewLikeHandler(svc)
	rec := invokeWithUser(t, 0, http.HandlerFunc(h.GetStats), http.MethodGet, "/api/v1/pins/8/likes", "", map[string]string{"pinID": "8"})
	if rec.Code != http.StatusInternalServerError {
		t.Fatalf("status=%d body=%s", rec.Code, rec.Body.String())
	}
}
