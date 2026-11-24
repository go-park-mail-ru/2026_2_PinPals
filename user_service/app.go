package user_service

import (
	"context"
	"log/slog"
	"net/http"
	"strings"
	"time"

	"2026_2_PinPals/pkg/config"
	"2026_2_PinPals/pkg/images"
	"2026_2_PinPals/pkg/middleware"
	"2026_2_PinPals/pkg/server"

	httphandler "2026_2_PinPals/user_service/delivery/http"
	"2026_2_PinPals/user_service/repository"
	"2026_2_PinPals/user_service/usecase"
	"github.com/jackc/pgx/v5/pgxpool"
)

func RegisterRoutes(cfg *config.Config, logger *slog.Logger, pool *pgxpool.Pool, mux *http.ServeMux) func() {
	tokens := middleware.NewTokenManager([]byte(cfg.Auth.JWTSecret), int64(cfg.Auth.TokenTTL/time.Second))
	repo := repository.NewRepository(pool)
	service := usecase.NewService(repo)
	handler := httphandler.NewHandler(service, httphandler.HandlerOptions{
		TokenRevoker:  tokens,
		SecureCookies: strings.HasPrefix(cfg.App.BaseURL, "https://"),

		BaseURL:      cfg.App.BaseURL,
		ImageStorage: images.NewStorage(cfg.App.ImageDir),
	})
	access := middleware.Access(tokens)
	mux.Handle("GET /api/v1/users/me", access(http.HandlerFunc(handler.GetMe)))
	mux.Handle("PATCH /api/v1/users/me", middleware.CSRF(access(http.HandlerFunc(handler.UpdateMe))))
	mux.Handle("DELETE /api/v1/users/me", middleware.CSRF(access(http.HandlerFunc(handler.DeleteMe))))
	return nil
}

func Run(ctx context.Context) error { return server.Run(ctx, RegisterRoutes) }
