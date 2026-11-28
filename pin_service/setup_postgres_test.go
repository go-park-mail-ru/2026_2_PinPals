package pin_service

import (
	"context"
	"io"
	"log/slog"
	"strings"
	"testing"

	"2026_2_PinPals/pkg/config"
	"2026_2_PinPals/pkg/database"
)

// This service-level regression test intentionally calls the shared pool
// constructor: database setup now belongs to pkg/server and pkg/database,
// not to a service-specific SetupPostgres wrapper.
func TestSharedPostgresPoolRejectsInvalidDSN(t *testing.T) {
	cfg := &config.Config{Postgres: config.PostgresConfig{
		Host: "[", Port: "5432", User: "user", Password: "pass", Database: "db", SSLMode: "disable",
	}}
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))

	if pool, err := database.NewPostgresPool(context.Background(), cfg.Postgres.DSN(), logger); err == nil || pool != nil || !strings.Contains(err.Error(), "create postgres pool") {
		t.Fatalf("NewPostgresPool() = %v, %v; expected pool creation error", pool, err)
	}
}
