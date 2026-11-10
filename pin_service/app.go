package pin_service

import (
	"context"
	"fmt"
	"log/slog"
	"net/http"
	"time"

	httphandler "2026_2_PinPals/pin_service/delivery/http"
	"2026_2_PinPals/pin_service/repository"
	"2026_2_PinPals/pin_service/usecase"
	"2026_2_PinPals/pkg/config"
	"2026_2_PinPals/pkg/images"
	"2026_2_PinPals/pkg/middleware"
	"2026_2_PinPals/pkg/server"

	"github.com/go-playground/validator/v10"
	"github.com/jackc/pgx/v5/pgxpool"
)

func SetupPostgres(ctx context.Context, cfg *config.Config, logger *slog.Logger) (*pgxpool.Pool, error) {
	pool, err := pgxpool.New(ctx, cfg.Postgres.DSN())
	if err != nil {
		logger.Error("failed to create postgres pool", "error", err)
		return nil, fmt.Errorf("create postgres pool: %w", err)
	}

	pingCtx, cancelPoolPing := context.WithTimeout(ctx, 5*time.Second)
	defer cancelPoolPing()
	if err := pool.Ping(pingCtx); err != nil {
		logger.Error("failed to connect to postgres", "error", err)
		pool.Close()
		return nil, fmt.Errorf("connect to postgres: %w", err)
	}

	return pool, nil
}

func RegisterRoutes(cfg *config.Config, logger *slog.Logger, pool *pgxpool.Pool, mux *http.ServeMux) {
	tokenManager := middleware.NewTokenManager(
		[]byte(cfg.Auth.JWTSecret),
		int64(cfg.Auth.TokenTTL/time.Second),
	)
	tokenManager.SetRevocationStore(middleware.NewPostgresTokenStore(pool))

	pinRepository := repository.NewRepository(pool)
	pinService := usecase.NewService(pinRepository)
	requestValidator := validator.New()
	pinHandler := httphandler.NewHandler(pinService, cfg.App.BaseURL, requestValidator, images.NewStorage(cfg.App.ImageDir))
	imageHandler := images.NewHandler(cfg.App.ImageDir)

	mux.HandleFunc("POST /api/v1/pins/search", pinHandler.List)
	mux.HandleFunc("GET /api/v1/pins/{pinID}", pinHandler.GetByID)
	mux.Handle(
		"POST /api/v1/pins",
		middleware.CSRF(middleware.Access(tokenManager)(http.HandlerFunc(pinHandler.Create))),
	)
	mux.Handle("PATCH /api/v1/pins/{pinID}", middleware.CSRF(middleware.Access(tokenManager)(http.HandlerFunc(pinHandler.Update))))

	mux.Handle("DELETE /api/v1/pins/{pinID}", middleware.CSRF(middleware.Access(tokenManager)(http.HandlerFunc(pinHandler.Delete))))
	deskService := usecase.NewDeskService(pinRepository)
	deskHandler := httphandler.NewDeskHandler(deskService, cfg.App.BaseURL, images.NewStorage(cfg.App.ImageDir))
	mux.Handle("POST /api/v1/desks", middleware.CSRF(middleware.Access(tokenManager)(http.HandlerFunc(deskHandler.Create))))
	mux.Handle("GET /api/v1/desks/me", middleware.Access(tokenManager)(http.HandlerFunc(deskHandler.ListOwn)))
	mux.HandleFunc("GET /api/v1/users/{userID}/desks", deskHandler.ListPublic)
	mux.Handle("GET /api/v1/desks/{deskID}", middleware.OptionalAccess(tokenManager)(http.HandlerFunc(deskHandler.GetByID)))
	mux.Handle("PATCH /api/v1/desks/{deskID}", middleware.CSRF(middleware.Access(tokenManager)(http.HandlerFunc(deskHandler.Update))))
	mux.Handle("DELETE /api/v1/desks/{deskID}", middleware.CSRF(middleware.Access(tokenManager)(http.HandlerFunc(deskHandler.Delete))))
	mux.Handle("POST /api/v1/desks/{deskID}/pins/{pinID}", middleware.CSRF(middleware.Access(tokenManager)(http.HandlerFunc(deskHandler.AddPin))))
	mux.Handle("DELETE /api/v1/desks/{deskID}/pins/{pinID}", middleware.CSRF(middleware.Access(tokenManager)(http.HandlerFunc(deskHandler.RemovePin))))
	likeService := usecase.NewLikeService(pinRepository)
	likeHandler := httphandler.NewLikeHandler(likeService)
	mux.Handle("POST /api/v1/pins/{pinID}/like", middleware.CSRF(middleware.Access(tokenManager)(http.HandlerFunc(likeHandler.Add))))
	mux.Handle("DELETE /api/v1/pins/{pinID}/like", middleware.CSRF(middleware.Access(tokenManager)(http.HandlerFunc(likeHandler.Remove))))
	mux.Handle("GET /api/v1/pins/{pinID}/likes", middleware.OptionalAccess(tokenManager)(http.HandlerFunc(likeHandler.GetStats)))
	mux.HandleFunc("GET /images/{filename}", imageHandler.Get)
}

func Run(ctx context.Context) error {
	return server.Run(ctx, SetupPostgres, RegisterRoutes)
}
