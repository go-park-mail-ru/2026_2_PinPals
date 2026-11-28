package main

import (
	"context"
	"errors"
	"log/slog"
	"net/http"
	"os"
	"time"

	"2026_2_PinPals/onlineconf_service/delivery"
	"2026_2_PinPals/pkg/config"
	"2026_2_PinPals/pkg/middleware"
	"2026_2_PinPals/pkg/server"
	"github.com/jackc/pgx/v5/pgxpool"
)

func run(ctx context.Context) error {
	secret := os.Getenv("ONLINECONF_INTERNAL_TOKEN")
	if len(secret) < 32 {
		return errors.New("ONLINECONF_INTERNAL_TOKEN must contain at least 32 characters")
	}
	service := &delivery.Server{}
	runCtx, cancel := context.WithCancel(ctx)
	defer cancel()
	setup := func(ctx context.Context, cfg *config.Config, _ *slog.Logger) (*pgxpool.Pool, error) {
		pool, err := pgxpool.New(ctx, cfg.Postgres.DSN())
		if err != nil {
			return nil, errors.New("database configuration invalid")
		}
		service.DB = pool
		if err = service.Refresh(ctx); err != nil {
			pool.Close()
			return nil, errors.New("cannot load OnlineConf tables; check database and migrations")
		}
		return pool, nil
	}
	routes := func(cfg *config.Config, _ *slog.Logger, pool *pgxpool.Pool, mux *http.ServeMux) {
		tm := middleware.NewTokenManager([]byte(cfg.Auth.JWTSecret), int64(cfg.Auth.TokenTTL/time.Second), middleware.WithRevocationStore(middleware.NewPostgresTokenStore(pool)))
		service.Routes(mux, tm, secret)
		go service.Poll(runCtx)
	}
	return server.Run(ctx, setup, routes)
}
func main() {
	if err := run(context.Background()); err != nil {
		slog.Error("onlineconf stopped", "error", err)
		os.Exit(1)
	}
}
