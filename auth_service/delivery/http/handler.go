package http

import (
	"context"
	"errors"
	"net/http"
	"time"

	"2026_2_PinPals/auth_service/domain"
	"2026_2_PinPals/pkg/httpx"
	"2026_2_PinPals/pkg/middleware"
)

type TokenIssuer func(userID int) (string, error)

type AuthService interface {
	Register(
		ctx context.Context,
		input domain.RegisterInput,
	) (domain.User, error)

	Login(
		ctx context.Context,
		input domain.LoginInput,
		issueToken func(userID int) (string, error),
	) (domain.AuthResponse, error)
}

//go:generate go run go.uber.org/mock/mockgen@v0.6.0 -destination=mocks/mock_token_revoker.go -package=mocks 2026_2_PinPals/auth_service/delivery/http TokenRevoker

type TokenRevoker interface {
	Revoke(context.Context, string) error
}

type HandlerOptions struct {
	TokenRevoker  TokenRevoker
	TokenTTL      time.Duration
	SecureCookies bool
}

type Handler struct {
	service     AuthService
	tokenIssuer TokenIssuer
	options     HandlerOptions
}

func NewHandler(service AuthService, tokenIssuer TokenIssuer, options ...HandlerOptions) *Handler {
	var opts HandlerOptions
	if len(options) > 0 {
		opts = options[0]
	}
	if opts.TokenTTL <= 0 {
		opts.TokenTTL = 24 * time.Hour
	}
	return &Handler{service: service, tokenIssuer: tokenIssuer, options: opts}
}

// Register godoc
// @Summary Регистрация пользователя
// @Tags Authentication
// @Accept json
// @Produce json
// @Param request body RegisterRequest true "Данные регистрации"
// @Success 201 {object} UserResponse
// @Failure 400 {object} map[string]interface{}
// @Failure 409 {object} map[string]interface{}
// @Router /auth/register [post]
func (h *Handler) Register(w http.ResponseWriter, r *http.Request) {
	var req RegisterRequest

	if err := httpx.DecodeJSON(w, r, &req); err != nil {
		return
	}

	input := req.ToDomain()

	user, err := h.service.Register(r.Context(), input)
	if err != nil {
		var validationErr *domain.ValidationErrors

		switch {
		case errors.As(err, &validationErr):
			httpx.WriteValidationError(
				w,
				http.StatusBadRequest,
				validationErr.Fields,
			)

		case errors.Is(err, domain.ErrUserTagExists):
			httpx.WriteJSON(
				w,
				http.StatusConflict,
				map[string]any{
					"error": "конфликт",
					"fields": map[string]string{
						"user_tag": "уже существует",
					},
				},
			)

		default:
			middleware.LoggerFromContext(r.Context()).ErrorContext(
				r.Context(),
				"не удалось зарегистрировать пользователя",
				"error", err,
			)

			httpx.WriteError(
				w,
				http.StatusInternalServerError,
				"не удалось зарегистрировать пользователя",
			)
		}

		return
	}

	httpx.WriteJSON(w, http.StatusCreated, UserResponseFromDomain(user))
}

// Login godoc
// @Summary Вход в аккаунт
// @Tags Authentication
// @Accept json
// @Produce json
// @Param request body LoginRequest true "Учётные данные"
// @Success 200 {object} AuthResponseDTO
// @Failure 400 {object} map[string]interface{}
// @Failure 401 {object} map[string]interface{}
// @Failure 500 {object} map[string]interface{}
// @Router /auth/login [post]
func (h *Handler) Login(w http.ResponseWriter, r *http.Request) {
	var req LoginRequest
	if err := httpx.DecodeJSON(w, r, &req); err != nil {
		return
	}

	input := req.ToDomain()

	response, err := h.service.Login(r.Context(), input, h.tokenIssuer)
	if err != nil {
		var validationErr *domain.ValidationErrors
		switch {
		case errors.As(err, &validationErr):
			httpx.WriteValidationError(w, http.StatusBadRequest, validationErr.Fields)
		case errors.Is(err, domain.ErrValidation):
			httpx.WriteError(w, http.StatusBadRequest, "заполните обязательные поля")
		case errors.Is(err, domain.ErrInvalidCredentials):
			httpx.WriteError(w, http.StatusUnauthorized, "неверный логин или пароль")
		default:
			middleware.LoggerFromContext(r.Context()).ErrorContext(
				r.Context(),
				"не удалось выполнить вход",
				"error", err,
			)

			httpx.WriteError(
				w,
				http.StatusInternalServerError,
				"не удалось выполнить вход",
			)
		}
		return
	}

	csrfToken, err := middleware.NewCSRFToken()
	if err != nil {
		middleware.LoggerFromContext(r.Context()).ErrorContext(r.Context(), "не удалось создать CSRF-токен", "error", err)
		httpx.WriteError(w, http.StatusInternalServerError, "не удалось выполнить вход")
		return
	}
	middleware.SetAuthCookies(w, response.Token, csrfToken, h.options.TokenTTL, h.options.SecureCookies)
	httpx.WriteJSON(w, http.StatusOK, AuthResponseFromDomain(response))
}

// Logout отзывает текущий JWT и очищает обе auth-cookie.
// Logout godoc
// @Summary Выход из аккаунта и отзыв текущего JWT
// @Tags Authentication
// @Produce json
// @Param Cookie header string true "Cookie access_token=...; csrf_token=..."
// @Param X-CSRF-Token header string true "CSRF-токен"
// @Success 204
// @Failure 401 {object} map[string]interface{}
// @Failure 403 {object} map[string]interface{}
// @Security CookieAuth
// @Router /auth/logout [post]
func (h *Handler) Logout(w http.ResponseWriter, r *http.Request) {
	token := middleware.TokenFromRequest(r)
	if token == "" {
		httpx.WriteError(w, http.StatusUnauthorized, "не авторизован")
		return
	}
	if h.options.TokenRevoker == nil {
		httpx.WriteError(w, http.StatusInternalServerError, "отзыв токенов не настроен")
		return
	}
	if err := h.options.TokenRevoker.Revoke(r.Context(), token); err != nil {
		middleware.LoggerFromContext(r.Context()).ErrorContext(r.Context(), "не удалось отозвать JWT", "error", err)
		httpx.WriteError(w, http.StatusInternalServerError, "не удалось выйти из аккаунта")
		return
	}
	middleware.ClearAuthCookies(w, h.options.SecureCookies)
	w.WriteHeader(http.StatusNoContent)
}
