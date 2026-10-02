package server

import (
	"log/slog"
	"net/http"

	"2026_2_PinPals/internal/auth"
	"2026_2_PinPals/internal/config"
	"2026_2_PinPals/internal/middleware"
	"2026_2_PinPals/internal/pins"
)

func NewRouter(
	cfg *config.Config,
	logger *slog.Logger,
	authHandler *auth.Handler,
	pinHandler *pins.Handler,
	tokenManager *middleware.TokenManager,
) http.Handler {
	mux := http.NewServeMux()

	mux.HandleFunc("GET /healthz", func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{"status":"ok"}`))
	})

	mux.HandleFunc("POST /api/v1/auth/register", authHandler.Register)
	mux.HandleFunc("POST /api/v1/auth/login", authHandler.Login)

	mux.HandleFunc("GET /api/v1/pins", pinHandler.List)
	mux.HandleFunc("GET /api/v1/pins/{pinID}", pinHandler.GetByID)

	mux.Handle(
		"POST /api/v1/pins",
		tokenManager.Middleware(http.HandlerFunc(pinHandler.Create)),
	)

	var handler http.Handler = mux
	handler = requestLogger(logger)(handler)
	handler = middleware.CORS(cfg.CORS.AllowedOrigins)(handler)

	return handler
}

func requestLogger(logger *slog.Logger) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			logger.Info("http request", "method", r.Method, "path", r.URL.Path)
			next.ServeHTTP(w, r)
		})
	}
}
