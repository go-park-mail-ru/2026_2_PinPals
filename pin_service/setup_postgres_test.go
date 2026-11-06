package pin_service

import (
	"context"
	"io"
	"log/slog"
	"strings"
	"testing"

	"2026_2_PinPals/pkg/config"
)

func TestSetupPostgresRejectsInvalidDSN(t *testing.T) {
	cfg := &config.Config{Postgres: config.PostgresConfig{Host: "[", Port: "5432", User: "user", Password: "pass", Database: "db", SSLMode: "disable"}}
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	if pool, err := SetupPostgres(context.Background(), cfg, logger); err == nil || pool != nil || !strings.Contains(err.Error(), "create postgres pool") {
		t.Fatalf("SetupPostgres() = %v, %v; expected pool creation error", pool, err)
	}
}

func TestSetupPostgresClosesPoolWhenPingContextIsCanceled(t *testing.T) {
	cfg := &config.Config{Postgres: config.PostgresConfig{Host: "127.0.0.1", Port: "5432", User: "user", Password: "pass", Database: "db", SSLMode: "disable"}}
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	pool, err := SetupPostgres(ctx, cfg, logger)
	if err == nil || pool != nil || !strings.Contains(err.Error(), "connect to postgres") {
		t.Fatalf("SetupPostgres() = %v, %v; expected ping error", pool, err)
	}
}
