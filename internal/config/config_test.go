package config

import (
	"os"
	"strings"
	"testing"
	"time"
)

func setValidEnv(t *testing.T) {
	t.Helper()
	t.Setenv("POSTGRES_USER", "user")
	t.Setenv("POSTGRES_PASSWORD", "pass word")
	t.Setenv("POSTGRES_DB", "db")
	t.Setenv("AUTH_JWT_SECRET", "12345678901234567890123456789012")
}

func mustPanic(t *testing.T, fn func()) {
	t.Helper()
	defer func() {
		if recover() == nil {
			t.Fatal("expected panic")
		}
	}()
	fn()
}

func TestMustLoadAndDefaults(t *testing.T) {
	setValidEnv(t)
	os.Unsetenv("APP_PORT")
	os.Unsetenv("APP_ENV")
	os.Unsetenv("LOG_LEVEL")
	os.Unsetenv("POSTGRES_HOST")
	os.Unsetenv("POSTGRES_INTERNAL_PORT")
	os.Unsetenv("POSTGRES_SSLMODE")
	os.Unsetenv("AUTH_TOKEN_TTL")
	os.Unsetenv("CORS_ALLOWED_ORIGINS")

	cfg := MustLoad()

	if cfg.App.Port != "8000" || cfg.App.Env != "development" || cfg.App.LogLevel != "info" {
		t.Fatalf("unexpected app defaults: %#v", cfg.App)
	}
	if cfg.Postgres.Host != "postgres" || cfg.Postgres.Port != "5432" || cfg.Postgres.SSLMode != "disable" {
		t.Fatalf("unexpected postgres defaults: %#v", cfg.Postgres)
	}
	if cfg.Auth.TokenTTL != 24*time.Hour {
		t.Fatalf("unexpected ttl: %v", cfg.Auth.TokenTTL)
	}
	if len(cfg.CORS.AllowedOrigins) != 1 || cfg.CORS.AllowedOrigins[0] != "http://localhost:3000" {
		t.Fatalf("unexpected cors defaults: %#v", cfg.CORS.AllowedOrigins)
	}
}

func TestMustLoadCustomValues(t *testing.T) {
	setValidEnv(t)
	t.Setenv("APP_PORT", "9000")
	t.Setenv("APP_ENV", "test")
	t.Setenv("LOG_LEVEL", "debug")
	t.Setenv("POSTGRES_HOST", "db")
	t.Setenv("POSTGRES_INTERNAL_PORT", "5433")
	t.Setenv("POSTGRES_SSLMODE", "require")
	t.Setenv("AUTH_TOKEN_TTL", "2h")
	t.Setenv("CORS_ALLOWED_ORIGINS", " http://a.example, ,http://b.example ")

	cfg := MustLoad()

	if cfg.App.Port != "9000" || cfg.App.Env != "test" || cfg.App.LogLevel != "debug" {
		t.Fatalf("unexpected app config: %#v", cfg.App)
	}
	if cfg.Postgres.Host != "db" || cfg.Postgres.Port != "5433" || cfg.Postgres.SSLMode != "require" {
		t.Fatalf("unexpected postgres config: %#v", cfg.Postgres)
	}
	if cfg.Auth.TokenTTL != 2*time.Hour {
		t.Fatalf("unexpected custom values: ttl=%v", cfg.Auth.TokenTTL)
	}
	if len(cfg.CORS.AllowedOrigins) != 2 {
		t.Fatalf("unexpected origins: %#v", cfg.CORS.AllowedOrigins)
	}
}

func TestMustLoadValidationPanics(t *testing.T) {
	tests := []struct {
		name  string
		setup func(*testing.T)
	}{
		{
			"missing postgres",
			func(t *testing.T) {
				setValidEnv(t)
				os.Unsetenv("POSTGRES_USER")
			},
		},
		{
			"missing jwt",
			func(t *testing.T) {
				setValidEnv(t)
				os.Unsetenv("AUTH_JWT_SECRET")
			},
		},
		{
			"short jwt",
			func(t *testing.T) {
				setValidEnv(t)
				t.Setenv("AUTH_JWT_SECRET", strings.Repeat("x", 31))
			},
		},
		{
			"bad duration",
			func(t *testing.T) {
				setValidEnv(t)
				t.Setenv("AUTH_TOKEN_TTL", "not-a-duration")
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tt.setup(t)
			mustPanic(t, func() { MustLoad() })
		})
	}
}

func TestDSNAndHelpers(t *testing.T) {
	p := PostgresConfig{
		User:     "user name",
		Password: "pass@word",
		Host:     "localhost",
		Port:     "5432",
		Database: "db",
		SSLMode:  "disable",
	}
	dsn := p.DSN()
	if !strings.Contains(dsn, "user+name") || !strings.Contains(dsn, "pass%40word") {
		t.Fatalf("unexpected dsn: %s", dsn)
	}

	t.Setenv("TEST_DURATION", "15m")
	if got := getEnvDuration("TEST_DURATION", time.Hour); got != 15*time.Minute {
		t.Fatalf("unexpected duration: %v", got)
	}
}
