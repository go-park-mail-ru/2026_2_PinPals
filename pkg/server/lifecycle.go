package server

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"2026_2_PinPals/pkg/config"
	"2026_2_PinPals/pkg/middleware"

	"github.com/jackc/pgx/v5/pgxpool"
)

func Run(
	ctx context.Context,
	setupDB func(context.Context, *config.Config, *slog.Logger) (*pgxpool.Pool, error),
	registerRoutes func(*config.Config, *slog.Logger, *pgxpool.Pool, *http.ServeMux),
) error {
	cfg := config.MustLoad()

	logger := slog.New(
		slog.NewJSONHandler(
			os.Stdout,
			&slog.HandlerOptions{
				Level: cfg.App.SlogLevel(),
			},
		),
	)

	pool, err := setupDB(ctx, cfg, logger)
	if err != nil {
		return fmt.Errorf("setup db: %w", err)
	}
	defer pool.Close()

	mux := http.NewServeMux()
	mux.HandleFunc("GET /healthz", func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{"status":"ok"}`))
	})
	registerRoutes(cfg, logger, pool, mux)

	var handler http.Handler = mux
	handler = RequestLogger(logger)(handler)
	handler = middleware.CORS(cfg.CORS.AllowedOrigins)(handler)

	httpServer := New(":"+cfg.App.Port, handler)

	shutdownCtx, stop := signal.NotifyContext(ctx, os.Interrupt, syscall.SIGTERM)
	defer stop()

	errCh := make(chan error, 1)
	go func() {
		logger.Info("server started", "port", cfg.App.Port)
		if err := httpServer.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			errCh <- err
		}
		errCh <- nil
	}()

	select {
	case <-shutdownCtx.Done():
	case err := <-errCh:
		if err != nil {
			logger.Error("server stopped unexpectedly", "error", err)
			return err
		}
	}

	shutdownRequestCtx, cancelShutdown := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancelShutdown()
	if err := httpServer.Shutdown(shutdownRequestCtx); err != nil {
		logger.Error("graceful shutdown failed", "error", err)
		return err
	}

	logger.Info("server stopped")
	return nil
}
