package delivery

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"2026_2_PinPals/pkg/middleware"
	sdk "2026_2_PinPals/pkg/onlineconf"
	"github.com/jackc/pgx/v5/pgxpool"
)

// The supplied connection is used only to create and drop a unique test database.
func TestPostgresAPI(t *testing.T) {
	dsn := os.Getenv("ONLINECONF_TEST_DSN")
	if dsn == "" {
		t.Skip("set ONLINECONF_TEST_DSN to a disposable PostgreSQL instance")
	}
	ctx := context.Background()
	admin, err := pgxpool.New(ctx, dsn)
	if err != nil {
		t.Fatal(err)
	}
	defer admin.Close()
	name := fmt.Sprintf("onlineconf_test_%d", time.Now().UnixNano())
	if _, err = admin.Exec(ctx, "CREATE DATABASE "+name); err != nil {
		t.Fatal(err)
	}
	defer func() {
		if _, err := admin.Exec(ctx, "DROP DATABASE "+name); err != nil {
			t.Error(err)
		}
	}()
	cfg := admin.Config()
	cfg.ConnConfig.Database = name
	pool, err := pgxpool.NewWithConfig(ctx, cfg)
	if err != nil {
		t.Fatal(err)
	}
	defer pool.Close()
	exec := func(sql string) {
		t.Helper()
		if _, err := pool.Exec(ctx, sql); err != nil {
			t.Fatal(err)
		}
	}
	migrations, err := filepath.Glob("../../db/migrations/*.sql")
	if err != nil || len(migrations) < 16 {
		t.Fatal("missing migrations", err)
	}
	for _, file := range migrations {
		data, err := os.ReadFile(file)
		if err != nil {
			t.Fatal(err)
		}
		sections := strings.Split(string(data), "---- create above / drop below ----")
		exec(sections[0])
		if strings.Contains(file, "016_onlineconf") {
			exec(sections[1])
			exec(sections[0])
		}
	}
	exec(`INSERT INTO onlineconf.user_role VALUES(1,'owner');
 INSERT INTO public."user"(name,user_tag,birth_date) VALUES('Reader','oc_reader','2000-01-01')`)
	service := &Server{DB: pool}
	if err = service.Refresh(ctx); err != nil {
		t.Fatal(err)
	}
	tm := middleware.NewTokenManager([]byte(strings.Repeat("a", 32)), 3600, middleware.WithRevocationStore(middleware.NewPostgresTokenStore(pool)))
	mux := http.NewServeMux()
	service.Routes(mux, tm, "test-secret")
	srv := httptest.NewServer(mux)
	defer srv.Close()
	base := "/api/v1/onlineconf/"
	call := func(user int, method, path, body string, csrf bool, want int) map[string]any {
		t.Helper()
		req, err := http.NewRequest(method, srv.URL+path, bytes.NewBufferString(body))
		if err != nil {
			t.Fatal(err)
		}
		if user != 0 {
			token, err := tm.Issue(user)
			if err != nil {
				t.Fatal(err)
			}
			req.AddCookie(&http.Cookie{Name: "access_token", Value: token})
			req.AddCookie(&http.Cookie{Name: "csrf_token", Value: "nonce"})
			if csrf {
				req.Header.Set("X-CSRF-Token", "nonce")
			}
		}
		res, err := srv.Client().Do(req)
		if err != nil {
			t.Fatal(err)
		}
		defer res.Body.Close()
		var out map[string]any
		_ = json.NewDecoder(res.Body).Decode(&out)
		if res.StatusCode != want {
			t.Fatalf("%s %s: got %d %v, want %d", method, path, res.StatusCode, out, want)
		}
		return out
	}
	endpoint := base + "admin/"
	call(0, "GET", endpoint+"me", "", false, 401)
	call(2, "GET", endpoint+"me", "", false, 403)
	call(1, "PATCH", endpoint+"configs/1", `{"value":40,"version":1}`, false, 403)
	call(1, "PATCH", endpoint+"configs/1", `{"value":101,"version":1}`, true, 422)
	call(1, "PATCH", endpoint+"configs/1", `{"value":40,"version":1}`, true, 200)
	call(1, "PATCH", endpoint+"configs/1", `{"value":20,"version":1}`, true, 409)
	call(1, "POST", endpoint+"configs", `{"key":"only.backend","title":"Internal","scopes":["pin_service"],"value_type":"boolean","value":false}`, true, 201)
	public := call(0, "GET", base+"client-state", "", false, 200)
	configs := public["configs"].(map[string]any)
	if _, leaked := configs["only.backend"]; leaked || configs[sdk.PageSizeKey] != float64(40) {
		t.Fatal(public)
	}
	call(0, "GET", "/internal/v1/onlineconf/snapshot?scope=pin_service", "", false, 403)
	req, _ := http.NewRequest("GET", srv.URL+base+"client-state", nil)
	first, err := srv.Client().Do(req)
	if err != nil {
		t.Fatal(err)
	}
	first.Body.Close()
	req.Header.Set("If-None-Match", first.Header.Get("ETag"))
	same, err := srv.Client().Do(req)
	if err != nil {
		t.Fatal(err)
	}
	same.Body.Close()
	if same.StatusCode != 304 || first.Header.Get("ETag") == "" {
		t.Fatal("conditional read", same.StatusCode)
	}
	var cache sdk.Cache
	if err = cache.Refresh(ctx, srv.Client(), srv.URL, "pin_service", "test-secret"); err != nil || cache.PageSize() != 40 {
		t.Fatal("SDK refresh", err)
	}
	call(1, "PATCH", endpoint+"flags/2", `{"allowlist":[1],"denylist":[1],"version":1}`, true, 422)
	call(1, "PATCH", endpoint+"flags/2", `{"denylist":[99999],"version":1}`, true, 422)
	call(1, "PATCH", endpoint+"flags/2", `{"rollout_percent":0,"allowlist":[2],"version":1}`, true, 200)
	decision := call(1, "POST", endpoint+"flags/2/evaluate", `{"user_id":2}`, true, 200)
	if decision["reason"] != "user_included" {
		t.Fatal(decision)
	}
	call(1, "POST", endpoint+"roles", `{"name":"operator","permissions":["flag.update"],"scopes":["frontend","pin_service"]}`, true, 200)
	call(2, "PATCH", endpoint+"flags/2", `{"active":false,"version":2}`, true, 403)
	call(1, "POST", endpoint+"groups", `{"name":"ops","roles":["operator"],"members":[2]}`, true, 200)
	call(2, "PATCH", endpoint+"flags/2", `{"active":false,"version":2}`, true, 200)
	call(1, "POST", endpoint+"groups", `{"name":"ops","roles":["operator"],"members":[]}`, true, 200)
	call(2, "PATCH", endpoint+"flags/2", `{"active":true,"version":3}`, true, 403)
	call(1, "POST", endpoint+"flags/2/rollout/create", `{"version":3,"plan":{"name":"staged","stages":[5,10,25,100],"duration_seconds":0}}`, true, 200)
	call(1, "PATCH", endpoint+"flags/2", `{"version":4,"rollout_percent":100}`, true, 409)
	result := call(1, "POST", endpoint+"flags/2/rollout/approve", `{"version":4}`, true, 200)
	if result["rollout_percent"] != float64(10) {
		t.Fatal(result)
	}
	var changed int
	err = pool.QueryRow(ctx, `SELECT count(*) FROM onlineconf.history WHERE resource_id='1' AND action='update' AND actor_id=1 AND version=2
 AND before_json->>'value'='30' AND after_json->>'value'='40'`).Scan(&changed)
	if err != nil || changed != 1 {
		t.Fatal("audit", changed, err)
	}
}
