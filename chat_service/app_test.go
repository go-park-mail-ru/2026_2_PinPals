package chat_service

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
		App:  config.AppConfig{BaseURL: "https://pins.example.test"},
		Auth: config.AuthConfig{JWTSecret: "01234567890123456789012345678901", TokenTTL: 24 * time.Hour},
		CORS: config.CORSConfig{AllowedOrigins: []string{"https://frontend.example.test"}},
	}
	mux := http.NewServeMux()
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	RegisterRoutes(cfg, logger, &pgxpool.Pool{}, mux)

	cases := []struct {
		method  string
		path    string
		pattern string
	}{
		{http.MethodPost, "/api/v1/chats", "POST /api/v1/chats"},
		{http.MethodGet, "/api/v1/chats", "GET /api/v1/chats"},
		{http.MethodGet, "/api/v1/chats/8/messages", "GET /api/v1/chats/{chatID}/messages"},
		{http.MethodPost, "/api/v1/chats/8/messages", "POST /api/v1/chats/{chatID}/messages"},
		{http.MethodGet, "/api/v1/chats/8/ws", "GET /api/v1/chats/{chatID}/ws"},
	}
	for _, tc := range cases {
		t.Run(tc.pattern, func(t *testing.T) {
			req := httptest.NewRequest(tc.method, tc.path, nil)
			_, pattern := mux.Handler(req)
			if pattern != tc.pattern {
				t.Fatalf("route %s %s resolved to %q, want %q", tc.method, tc.path, pattern, tc.pattern)
			}
		})
	}
}
