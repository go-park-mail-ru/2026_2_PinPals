package pin_service

import (
	"context"
	"fmt"
	"log/slog"
	"net/http"
	"time"

	httphandler "2026_2_PinPals/pin_service/delivery/http"
	"2026_2_PinPals/pin_service/repository"
	"2026_2_PinPals/pin_service/usecase"
	"2026_2_PinPals/pkg/config"
	"2026_2_PinPals/pkg/images"
	"2026_2_PinPals/pkg/middleware"
	"2026_2_PinPals/pkg/server"

	"github.com/jackc/pgx/v5/pgxpool"
)

func SetupPostgres(ctx context.Context, cfg *config.Config, logger *slog.Logger) (*pgxpool.Pool, error) {
	pool, err := pgxpool.New(ctx, cfg.Postgres.DSN())
	if err != nil {
		logger.Error("failed to create postgres pool", "error", err)
		return nil, fmt.Errorf("create postgres pool: %w", err)
	}

	pingCtx, cancelPoolPing := context.WithTimeout(ctx, 5*time.Second)
	defer cancelPoolPing()
	if err := pool.Ping(pingCtx); err != nil {
		logger.Error("failed to connect to postgres", "error", err)
		pool.Close()
		return nil, fmt.Errorf("connect to postgres: %w", err)
	}

	return pool, nil
}

func RegisterRoutes(cfg *config.Config, logger *slog.Logger, pool *pgxpool.Pool, mux *http.ServeMux) {
	tokenManager := middleware.NewTokenManager(
		[]byte(cfg.Auth.JWTSecret),
		int64(cfg.Auth.TokenTTL/time.Second),
	)

	pinRepository := repository.NewRepository(pool)
	pinService := usecase.NewService(pinRepository)
	pinHandler := httphandler.NewHandler(pinService, logger, cfg.App.BaseURL)
	imageHandler := images.NewHandler(cfg.App.ImageDir)

	mux.HandleFunc("POST /api/v1/pins/search", pinHandler.List)
	mux.HandleFunc("GET /api/v1/pins/{pinID}", pinHandler.GetByID)
	mux.Handle(
		"POST /api/v1/pins",
		tokenManager.Middleware(http.HandlerFunc(pinHandler.Create)),
	)
	mux.HandleFunc("GET /images/{filename}", imageHandler.Get)
}

func Run(ctx context.Context) error {
	return server.Run(ctx, SetupPostgres, RegisterRoutes)
}
