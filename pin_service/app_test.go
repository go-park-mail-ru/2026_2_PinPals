package pin_service

import (
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"2026_2_PinPals/pkg/config"
	"github.com/jackc/pgx/v5/pgxpool"
)

func TestRegisterRoutes(t *testing.T) {
	cfg := &config.Config{
		App:  config.AppConfig{BaseURL: "https://pins.example.test", ImageDir: t.TempDir()},
		Auth: config.AuthConfig{JWTSecret: "01234567890123456789012345678901", TokenTTL: 24 * time.Hour},
	}
	mux := http.NewServeMux()
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	RegisterRoutes(cfg, logger, &pgxpool.Pool{}, mux)

	cases := []struct {
		method  string
		path    string
		pattern string
	}{
		{http.MethodPost, "/api/v1/pins/search", "POST /api/v1/pins/search"},
		{http.MethodGet, "/api/v1/pins/8", "GET /api/v1/pins/{pinID}"},
		{http.MethodPost, "/api/v1/pins", "POST /api/v1/pins"},
		{http.MethodPost, "/api/v1/desks", "POST /api/v1/desks"},
		{http.MethodGet, "/api/v1/desks/me", "GET /api/v1/desks/me"},
		{http.MethodGet, "/api/v1/users/3/desks", "GET /api/v1/users/{userID}/desks"},
		{http.MethodGet, "/api/v1/desks/4", "GET /api/v1/desks/{deskID}"},
		{http.MethodPatch, "/api/v1/desks/4", "PATCH /api/v1/desks/{deskID}"},
		{http.MethodDelete, "/api/v1/desks/4", "DELETE /api/v1/desks/{deskID}"},
		{http.MethodPost, "/api/v1/desks/4/pins/8", "POST /api/v1/desks/{deskID}/pins/{pinID}"},
		{http.MethodDelete, "/api/v1/desks/4/pins/8", "DELETE /api/v1/desks/{deskID}/pins/{pinID}"},
		{http.MethodPost, "/api/v1/pins/8/like", "POST /api/v1/pins/{pinID}/like"},
		{http.MethodDelete, "/api/v1/pins/8/like", "DELETE /api/v1/pins/{pinID}/like"},
		{http.MethodGet, "/api/v1/pins/8/likes", "GET /api/v1/pins/{pinID}/likes"},
		{http.MethodGet, "/images/photo.jpg", "GET /images/{filename}"},
	}
	for _, tc := range cases {
		t.Run(tc.pattern, func(t *testing.T) {
			req := httptest.NewRequest(tc.method, tc.path, nil)
			_, pattern := mux.Handler(req)
			if pattern != tc.pattern {
				t.Fatalf("route %s %s resolved to %q", tc.method, tc.path, pattern)
			}
		})
	}
}
