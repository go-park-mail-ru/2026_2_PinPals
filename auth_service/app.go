package auth_service

import (
	"context"
	"log/slog"
	"net/http"
	"strings"
	"time"

	httphandler "2026_2_PinPals/auth_service/delivery/http"
	"2026_2_PinPals/auth_service/repository"
	"2026_2_PinPals/auth_service/usecase"
	"2026_2_PinPals/pkg/config"
	"2026_2_PinPals/pkg/middleware"
	"2026_2_PinPals/pkg/server"

	"github.com/jackc/pgx/v5/pgxpool"
)

func RegisterRoutes(cfg *config.Config, logger *slog.Logger, pool *pgxpool.Pool, mux *http.ServeMux) func() {
	tokenManager := middleware.NewTokenManager(
		[]byte(cfg.Auth.JWTSecret),
		int64(cfg.Auth.TokenTTL/time.Second),
		middleware.WithRevocationStore(middleware.NewPostgresTokenStore(pool)),
	)

	authRepository := repository.NewRepository(pool)
	authService := usecase.NewService(authRepository)
	authHandler := httphandler.NewHandler(authService, tokenManager.Issue, httphandler.HandlerOptions{TokenRevoker: tokenManager, TokenTTL: cfg.Auth.TokenTTL, SecureCookies: strings.HasPrefix(cfg.App.BaseURL, "https://"), BaseURL: cfg.App.BaseURL})

	mux.HandleFunc("POST /api/v1/auth/register", authHandler.Register)
	mux.HandleFunc("POST /api/v1/auth/login", authHandler.Login)
	mux.Handle("POST /api/v1/auth/logout", middleware.CSRF(middleware.Access(tokenManager)(http.HandlerFunc(authHandler.Logout))))
	return nil
}

func Run(ctx context.Context) error {
	return server.Run(ctx, RegisterRoutes)
}
