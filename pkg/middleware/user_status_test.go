package middleware

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

type userStatusStore struct {
	deleted map[int]bool
}

func (s userStatusStore) Revoke(context.Context, string, time.Time) error { return nil }
func (s userStatusStore) IsRevoked(context.Context, string) (bool, error) { return false, nil }
func (s userStatusStore) IsUserDeleted(_ context.Context, id int) (bool, error) {
	return s.deleted[id], nil
}

func TestMiddlewareRejectsTokenOfDeletedUser(t *testing.T) {
	manager := NewTokenManager([]byte("01234567890123456789012345678901"), int64(time.Hour/time.Second))
	manager.SetRevocationStore(userStatusStore{deleted: map[int]bool{7: true}})
	next := http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) { w.WriteHeader(http.StatusNoContent) })

	for _, tc := range []struct {
		userID int
		want   int
	}{{7, http.StatusUnauthorized}, {8, http.StatusNoContent}} {
		token, err := manager.Issue(tc.userID)
		if err != nil {
			t.Fatal(err)
		}
		for name, mw := range map[string]func(http.Handler) http.Handler{"required": manager.Middleware, "optional": manager.OptionalMiddleware} {
			req := httptest.NewRequest(http.MethodGet, "/", nil)
			req.Header.Set("Authorization", "Bearer "+token)
			rec := httptest.NewRecorder()
			mw(next).ServeHTTP(rec, req)
			if rec.Code != tc.want {
				t.Fatalf("%s middleware, user %d: expected %d, got %d", name, tc.userID, tc.want, rec.Code)
			}
		}
	}
}
