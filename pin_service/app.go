package pin_service

import (
	"context"
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

func RegisterRoutes(cfg *config.Config, logger *slog.Logger, pool *pgxpool.Pool, mux *http.ServeMux) func() {
	tokenManager := middleware.NewTokenManager(
		[]byte(cfg.Auth.JWTSecret),
		int64(cfg.Auth.TokenTTL/time.Second),
	)

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
	deskService := usecase.NewDeskService(pinRepository, images.NewStorage(cfg.App.ImageDir))
	deskHandler := httphandler.NewDeskHandler(deskService, cfg.App.BaseURL)
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
	mux.Handle("GET /api/v1/desks/{deskID}/pins/preview", middleware.OptionalAccess(tokenManager)(http.HandlerFunc(deskHandler.GetFirstPins)))
	return nil
}

func Run(ctx context.Context) error {
	return server.Run(ctx, RegisterRoutes)
}
