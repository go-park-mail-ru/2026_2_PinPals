package config

import (
	"fmt"
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
	URL string
}

type MinIOConfig struct {
	Endpoint  string
	AccessKey string
	SecretKey string
	Bucket    string
	UseSSL    bool
	Region    string
}

// Load читает переменные окружения и возвращает Config.
// Если чего-то обязательного нет — возвращает ошибку.
func Load() (*Config, error) {
	cfg := &Config{
		App: AppConfig{
			Port:     getEnv("APP_PORT", "8000"),
			Env:      getEnv("APP_ENV", "development"),
			LogLevel: getEnv("LOG_LEVEL", "info"),
		},
		Postgres: PostgresConfig{
			URL: getEnv("DATABASE_URL", ""),
		},
		MinIO: MinIOConfig{
			Endpoint:  getEnv("S3_ENDPOINT", ""),
			AccessKey: getEnv("S3_ACCESS_KEY", ""),
			SecretKey: getEnv("S3_SECRET_KEY", ""),
			Bucket:    getEnv("S3_BUCKET", ""),
			UseSSL:    getEnvBool("S3_USE_SSL", false),
			Region:    getEnv("S3_REGION", "us-east-1"),
		},
	}

	if cfg.Postgres.URL == "" {
		return nil, fmt.Errorf("DATABASE_URL is required")
	}
	if cfg.MinIO.Endpoint == "" || cfg.MinIO.AccessKey == "" || cfg.MinIO.SecretKey == "" {
		return nil, fmt.Errorf("S3_ENDPOINT, S3_ACCESS_KEY, S3_SECRET_KEY are required")
	}

	return cfg, nil
}

func getEnv(key, fallback string) string {
	v, ok := os.LookupEnv(key);
	if ok && v != "" {
		return v
	}
	return fallback
}

func getEnvBool(key string, fallback bool) bool {
	value, ok := os.LookupEnv(key)
	if !ok || value == "" {
		return fallback
	}
	boolValue, err := strconv.ParseBool(value)
	if err != nil {
		return fallback
	}
	return boolValue
}
