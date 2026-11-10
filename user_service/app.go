package user_service

import (
	"context"
	"fmt"
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

func SetupPostgres(ctx context.Context, cfg *config.Config, logger *slog.Logger) (*pgxpool.Pool, error) {
	pool, err := pgxpool.New(ctx, cfg.Postgres.DSN())
	if err != nil {
		logger.Error("failed to create postgres pool", "error", err)
		return nil, fmt.Errorf("create postgres pool: %w", err)
	}
	pingCtx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()
	if err := pool.Ping(pingCtx); err != nil {
		logger.Error("failed to connect to postgres", "error", err)
		pool.Close()
		return nil, fmt.Errorf("connect to postgres: %w", err)
	}
	return pool, nil
}

func RegisterRoutes(cfg *config.Config, logger *slog.Logger, pool *pgxpool.Pool, mux *http.ServeMux) {
	tokens := middleware.NewTokenManager([]byte(cfg.Auth.JWTSecret), int64(cfg.Auth.TokenTTL/time.Second))
	tokens.SetRevocationStore(middleware.NewPostgresTokenStore(pool))
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
}

func Run(ctx context.Context) error { return server.Run(ctx, SetupPostgres, RegisterRoutes) }
