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
	registerRoutes func(*config.Config, *slog.Logger, *pgxpool.Pool, *http.ServeMux) func(),
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
	shutdownRoutes := registerRoutes(cfg, logger, pool, mux)

	var handler http.Handler = mux
	handler = middleware.CORS(cfg.CORS.AllowedOrigins)(handler)
	handler = middleware.SecurityHeaders(handler)
	handler = RequestLogger(logger)(handler)

	httpServer := New(":"+cfg.App.Port, handler)
	if shutdownRoutes != nil {
		httpServer.RegisterOnShutdown(shutdownRoutes)
	}

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
			_ = httpServer.Close()
			if shutdownRoutes != nil {
				shutdownRoutes()
			}
			return err
		}
	}

	shutdownRequestCtx, cancelShutdown := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancelShutdown()
	shutdownErr := httpServer.Shutdown(shutdownRequestCtx)
	if shutdownErr != nil {
		// Force-close ordinary HTTP connections after the graceful timeout.
		// Hijacked connections are closed by the route-specific callback below.
		_ = httpServer.Close()
	}
	// net/http invokes RegisterOnShutdown callbacks asynchronously and does not
	// wait for hijacked connections such as WebSockets. Invoke the route cleanup
	// callback again after Shutdown; chat hub cleanup is idempotent and waits for
	// all upgraded handlers to finish before the deferred pool.Close runs.
	if shutdownRoutes != nil {
		shutdownRoutes()
	}
	if shutdownErr != nil {
		logger.Error("graceful shutdown failed", "error", shutdownErr)
		return shutdownErr
	}

	logger.Info("server stopped")
	return nil
}
