package config

import (
	"fmt"
	"net/url"
	"os"
	"strconv"
)

// Config — вся конфигурация приложения, собранная из переменных окружения.
type Config struct {
	App      AppConfig
	Postgres PostgresConfig
	MinIO    MinIOConfig
}

type AppConfig struct {
	Port     string
	Env      string
	LogLevel string
}

type PostgresConfig struct {
	Host     string
	Port     string
	User     string
	Password string
	Database string
	SSLMode  string
}

type MinIOConfig struct {
	Endpoint  string
	AccessKey string
	SecretKey string
	Bucket    string
	UseSSL    bool
}

// MustLoad читает переменные окружения и возвращает Config.
// Если чего-то обязательного нет — выкидывает панику.
func MustLoad() *Config {
	cfg := &Config{
		App: AppConfig{
			Port:     getEnv("APP_PORT", "8000"),
			Env:      getEnv("APP_ENV", "development"),
			LogLevel: getEnv("LOG_LEVEL", "info"),
		},
		Postgres: PostgresConfig{
			Host:     getEnv("POSTGRES_HOST", "postgres"),
			Port:     getEnv("POSTGRES_INTERNAL_PORT", "5432"),
			User:     getEnv("POSTGRES_USER", ""),
			Password: getEnv("POSTGRES_PASSWORD", ""),
			Database: getEnv("POSTGRES_DB", ""),
			SSLMode:  getEnv("POSTGRES_SSLMODE", "disable"),
		},
		MinIO: MinIOConfig{
			Endpoint:  getEnv("S3_ENDPOINT", ""),
			AccessKey: getEnv("S3_ACCESS_KEY", ""),
			SecretKey: getEnv("S3_SECRET_KEY", ""),
			Bucket:    getEnv("S3_BUCKET", ""),
			UseSSL:    mustGetEnvBool("S3_USE_SSL", false),
		},
	}

	if cfg.Postgres.User == "" || cfg.Postgres.Password == "" || cfg.Postgres.Database == "" {
		panic("POSTGRES_USER, POSTGRES_PASSWORD, POSTGRES_DB are required")
	}
	if cfg.MinIO.Endpoint == "" || cfg.MinIO.AccessKey == "" || cfg.MinIO.SecretKey == "" || cfg.MinIO.Bucket == "" {
		panic("S3_ENDPOINT, S3_ACCESS_KEY, S3_SECRET_KEY, Bucket are required")
	}

	return cfg
}

func (p PostgresConfig) DSN() string {
	return fmt.Sprintf(
		"postgres://%s:%s@%s:%s/%s?sslmode=%s",
		url.QueryEscape(p.User),
		url.QueryEscape(p.Password),
		p.Host,
		p.Port,
		p.Database,
		p.SSLMode,
	)
}

func getEnv(key, fallback string) string {
	value, ok := os.LookupEnv(key)
	if ok && value != "" {
		return value
	}
	return fallback
}

func getEnvBool(key string, fallback bool) (bool, error) {
	value, ok := os.LookupEnv(key)
	if !ok || value == "" {
		return fallback, nil
	}
	boolValue, err := strconv.ParseBool(value)
	if err != nil {
		return false, fmt.Errorf("invalid boolean value for %s=%q: %w", key, value, err)
	}
	return boolValue, nil
}

func mustGetEnvBool(key string, fallback bool) bool {
	value, err := getEnvBool(key, fallback)
	if err != nil {
		panic(err)
	}
	return value
}
