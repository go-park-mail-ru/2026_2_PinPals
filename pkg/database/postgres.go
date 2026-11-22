package database

import (
	"context"
	"fmt"
	"log/slog"
	"time"

	"2026_2_PinPals/pkg/middleware"
	"github.com/jackc/pgx/v5/pgxpool"
)

// NewPostgresPool creates a pool with request-aware SQL tracing and verifies
// connectivity before returning it to the service.
func NewPostgresPool(ctx context.Context, dsn string, logger *slog.Logger) (*pgxpool.Pool, error) {
	poolConfig, err := pgxpool.ParseConfig(dsn)
	if err != nil {
		if logger != nil {
			logger.Error("failed to parse postgres config", "error", err)
		}
		return nil, fmt.Errorf("create postgres pool: %w", err)
	}
	poolConfig.ConnConfig.Tracer = middleware.NewQueryLogger()

	pool, err := pgxpool.NewWithConfig(ctx, poolConfig)
	if err != nil {
		if logger != nil {
			logger.Error("failed to create postgres pool", "error", err)
		}
		return nil, fmt.Errorf("create postgres pool: %w", err)
	}

	pingCtx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()
	if err := pool.Ping(pingCtx); err != nil {
		if logger != nil {
			logger.Error("failed to connect to postgres", "error", err)
		}
		pool.Close()
		return nil, fmt.Errorf("connect to postgres: %w", err)
	}
	return pool, nil
}
