package auth_service

import (
	"context"
	"fmt"
	"log/slog"
	"net/http"
	"time"

	httphandler "2026_2_PinPals/auth_service/delivery/http"
	"2026_2_PinPals/auth_service/repository"
	"2026_2_PinPals/auth_service/usecase"
	"2026_2_PinPals/pkg/config"
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

	authRepository := repository.NewRepository(pool)
	authService := usecase.NewService(authRepository)
	authHandler := httphandler.NewHandler(authService, tokenManager.Issue, logger)

	mux.HandleFunc("POST /api/v1/auth/register", authHandler.Register)
	mux.HandleFunc("POST /api/v1/auth/login", authHandler.Login)
}

func Run(ctx context.Context) error {
	return server.Run(ctx, SetupPostgres, RegisterRoutes)
}
