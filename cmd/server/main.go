package main

import (
	"context"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"2026_2_PinPals/internal/auth"
	"2026_2_PinPals/internal/config"
	"2026_2_PinPals/internal/middleware"
	"2026_2_PinPals/internal/pins"
	"2026_2_PinPals/internal/server"

	"github.com/jackc/pgx/v5/pgxpool"
)

func main() {
	cfg := config.MustLoad()
	logger := slog.New(slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{Level: slog.LevelInfo}))

	ctx := context.Background()
	pool, err := pgxpool.New(ctx, cfg.Postgres.DSN())
	if err != nil {
		logger.Error("failed to create postgres pool", "error", err)
		os.Exit(1)
	}
	defer pool.Close()

	pingCtx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()
	if err := pool.Ping(pingCtx); err != nil {
		logger.Error("failed to connect to postgres", "error", err)
		os.Exit(1)
	}

	authRepository := auth.NewRepository(pool)
	authService := auth.NewService(authRepository)

	tokenManager := middleware.NewTokenManager([]byte(cfg.Auth.JWTSecret), int64(cfg.Auth.TokenTTL/time.Second))
	authHandler := auth.NewHandler(authService, tokenManager.Issue)

	pinRepository := pins.NewRepository(pool)
	pinService := pins.NewService(pinRepository)
	pinHandler := pins.NewHandler(pinService)

	handler := server.NewRouter(cfg, logger, authHandler, pinHandler, tokenManager)

	httpServer := &http.Server{
		Addr:              ":" + cfg.App.Port,
		Handler:           handler,
		ReadHeaderTimeout: 5 * time.Second,
		ReadTimeout:       10 * time.Second,
		WriteTimeout:      15 * time.Second,
		IdleTimeout:       60 * time.Second,
	}

	shutdownCtx, stop := signal.NotifyContext(ctx, os.Interrupt, syscall.SIGTERM)
	defer stop()

	go func() {
		logger.Info("server started", "port", cfg.App.Port)
		if err := httpServer.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			logger.Error("server stopped unexpectedly", "error", err)
			os.Exit(1)
		}
	}()

	<-shutdownCtx.Done()
	shutdownRequestCtx, cancelShutdown := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancelShutdown()
	if err := httpServer.Shutdown(shutdownRequestCtx); err != nil {
		logger.Error("graceful shutdown failed", "error", err)
	}
}
