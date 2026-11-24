package chat_service

import (
	"context"
	"log/slog"
	"net/http"
	"time"

	httphandler "2026_2_PinPals/chat_service/delivery/http"
	"2026_2_PinPals/chat_service/repository"
	"2026_2_PinPals/chat_service/usecase"
	"2026_2_PinPals/pkg/config"
	"2026_2_PinPals/pkg/middleware"
	"2026_2_PinPals/pkg/server"
	"github.com/jackc/pgx/v5/pgxpool"
)

func RegisterRoutes(cfg *config.Config, _ *slog.Logger, pool *pgxpool.Pool, mux *http.ServeMux) func() {
	tokens := middleware.NewTokenManager([]byte(cfg.Auth.JWTSecret), int64(cfg.Auth.TokenTTL/time.Second))
	repo := repository.NewRepository(pool)
	service := usecase.NewService(repo)
	hub := httphandler.NewHub()
	handler := httphandler.NewHandler(service, hub, httphandler.HandlerOptions{
		AllowedOrigins: cfg.CORS.AllowedOrigins,
		BaseURL:        cfg.App.BaseURL,
	})
	access := middleware.Access(tokens)

	mux.Handle("POST /api/v1/chats", middleware.CSRF(access(http.HandlerFunc(handler.CreateChat))))
	mux.Handle("GET /api/v1/chats", access(http.HandlerFunc(handler.ListChats)))
	mux.Handle("GET /api/v1/chats/{chatID}/messages", access(http.HandlerFunc(handler.ListMessages)))
	mux.Handle("POST /api/v1/chats/{chatID}/messages", middleware.CSRF(access(http.HandlerFunc(handler.SendMessage))))
	mux.Handle("GET /api/v1/chats/{chatID}/ws", access(http.HandlerFunc(handler.WebSocket)))
	return hub.CloseAll
}

func Run(ctx context.Context) error {
	return server.Run(ctx, RegisterRoutes)
}
