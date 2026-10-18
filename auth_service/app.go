package auth_service

import (
	"context"
	"log/slog"
	"net/http"
	"time"

	"2026_2_PinPals/auth_service/delivery"
	"2026_2_PinPals/auth_service/repository"
	"2026_2_PinPals/auth_service/usecase"
	"2026_2_PinPals/pkg/config"
	"2026_2_PinPals/pkg/middleware"

	"github.com/jackc/pgx/v5/pgxpool"
)

func Start(
	ctx context.Context,
	cfg *config.Config,
	logger *slog.Logger,
	pool *pgxpool.Pool,
	mux *http.ServeMux,
) {
	tokenManager := middleware.NewTokenManager(
		[]byte(cfg.Auth.JWTSecret),
		int64(cfg.Auth.TokenTTL/time.Second),
	)

	authRepository := repository.NewRepository(pool)
	authService := usecase.NewService(authRepository)
	authHandler := delivery.NewHandler(authService, tokenManager.Issue, logger)

	mux.HandleFunc("POST /api/v1/auth/register", authHandler.Register)
	mux.HandleFunc("POST /api/v1/auth/login", authHandler.Login)
}
