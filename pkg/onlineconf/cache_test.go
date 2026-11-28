package onlineconf

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestDecisionOrder(t *testing.T) {
	flag := Flag{ID: 5, Active: true, Percent: 50, Allow: []int{12}, Deny: []int{13}}
	for _, tc := range []struct {
		user   int
		reason string
	}{{12, "user_included"}, {13, "user_excluded"}, {0, "rollout_miss"}} {
		if got := flag.Evaluate(tc.user).Reason; got != tc.reason {
			t.Fatalf("%d: %s", tc.user, got)
		}
	}
	for user := 1; user < 100; user++ {
		previous := -1
		for p := 0; p <= 100; p++ {
			f := Flag{ID: 97, Active: true, Percent: p}
			result := f.Evaluate(user)
			if previous >= 0 && previous != result.Bucket {
				t.Fatal("unstable bucket")
			}
			previous = result.Bucket
			if result.Enabled != (result.Bucket < p) {
				t.Fatal("wrong cohort")
			}
		}
	}
	flag.Active = false
	if flag.Evaluate(12).Enabled {
		t.Fatal("override bypassed global switch")
	}
}
func TestCacheKeepsLastGoodSnapshot(t *testing.T) {
	var c Cache
	if c.Enabled(LikesKey, 1) || c.PageSize() != 30 {
		t.Fatal("cold fallback")
	}
	response := `{"scope":"pin_service","revision":3,"configs":{"pins.pagination.default_page_size":42},"flags":{"pins.likes.enabled":{"id":2,"active":true,"rollout_percent":100}}}`
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("Authorization") != "Bearer local" || r.URL.Query().Get("scope") != "pin_service" {
			t.Error("invalid SDK request")
		}
		fmt.Fprint(w, response)
	}))
	defer srv.Close()
	sync := func() error { return c.Refresh(context.Background(), srv.Client(), srv.URL, "pin_service", "local") }
	if err := sync(); err != nil {
		t.Fatal(err)
	}
	for _, invalid := range []string{"{", "{}", `{"scope":"user_service","revision":3,"configs":{},"flags":{}}`} {
		response = invalid
		if sync() == nil {
			t.Fatal("accepted invalid snapshot")
		}
	}
	srv.Close()
	if sync() == nil {
		t.Fatal("expected network failure")
	}
	if !c.Enabled(LikesKey, 1) || c.PageSize() != 42 {
		t.Fatal("last good snapshot lost")
	}
}
