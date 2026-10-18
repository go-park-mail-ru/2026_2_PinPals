package pin_service

import (
	"log/slog"
	"net/http"
	"time"

	"2026_2_PinPals/pin_service/delivery"
	"2026_2_PinPals/pin_service/repository"
	"2026_2_PinPals/pin_service/usecase"
	"2026_2_PinPals/pkg/config"
	"2026_2_PinPals/pkg/images"
	"2026_2_PinPals/pkg/middleware"

	"github.com/jackc/pgx/v5/pgxpool"
)

func Start(
	cfg *config.Config,
	logger *slog.Logger,
	pool *pgxpool.Pool,
	mux *http.ServeMux,
) {
	tokenManager := middleware.NewTokenManager(
		[]byte(cfg.Auth.JWTSecret),
		int64(cfg.Auth.TokenTTL/time.Second),
	)

	pinRepository := repository.NewRepository(pool)
	pinService := usecase.NewService(pinRepository)
	pinHandler := delivery.NewHandler(pinService, logger, cfg.App.BaseURL)
	imageHandler := images.NewHandler(cfg.App.ImageDir)

	mux.HandleFunc("POST /api/v1/pins/search", pinHandler.List)
	mux.HandleFunc("GET /api/v1/pins/{pinID}", pinHandler.GetByID)
	mux.Handle(
		"POST /api/v1/pins",
		tokenManager.Middleware(http.HandlerFunc(pinHandler.Create)),
	)
	mux.HandleFunc("GET /images/{filename}", imageHandler.Get)
}
