package auth_service

import (
	"context"
	"fmt"
	"log/slog"
	"net/http"
	"strings"
	"time"

	httphandler "2026_2_PinPals/auth_service/delivery/http"
	"2026_2_PinPals/auth_service/repository"
	"2026_2_PinPals/auth_service/usecase"
	_ "2026_2_PinPals/docs/auth"
	"2026_2_PinPals/pkg/config"
	"2026_2_PinPals/pkg/middleware"
	"2026_2_PinPals/pkg/server"

	httpSwagger "github.com/swaggo/http-swagger/v2"

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
	tokenManager.SetRevocationStore(middleware.NewPostgresTokenStore(pool))

	authRepository := repository.NewRepository(pool)
	authService := usecase.NewService(authRepository)
	authHandler := httphandler.NewHandler(authService, tokenManager.Issue, httphandler.HandlerOptions{TokenRevoker: tokenManager, TokenTTL: cfg.Auth.TokenTTL, SecureCookies: strings.HasPrefix(cfg.App.BaseURL, "https://")})

	mux.HandleFunc("POST /api/v1/auth/register", authHandler.Register)
	mux.HandleFunc("POST /api/v1/auth/login", authHandler.Login)
	mux.Handle("POST /api/v1/auth/logout", middleware.CSRF(middleware.Access(tokenManager)(http.HandlerFunc(authHandler.Logout))))
	if !cfg.App.IsProduction() {
		mux.Handle("GET /swagger/", httpSwagger.Handler(httpSwagger.URL("/swagger/doc.json")))
	}
}

func Run(ctx context.Context) error {
	return server.Run(ctx, SetupPostgres, RegisterRoutes)
}
