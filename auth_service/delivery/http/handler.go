package http

import (
	"context"
	"errors"
	"net/http"

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

type Handler struct {
	service     AuthService
	tokenIssuer TokenIssuer
}

func NewHandler(service AuthService, tokenIssuer TokenIssuer) *Handler {
	return &Handler{service: service, tokenIssuer: tokenIssuer}
}

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

func (h *Handler) Login(w http.ResponseWriter, r *http.Request) {
	var req LoginRequest
	if err := httpx.DecodeJSON(w, r, &req); err != nil {
		return
	}

	input := req.ToDomain()

	response, err := h.service.Login(r.Context(), input, h.tokenIssuer)
	if err != nil {
		switch {
		case errors.Is(err, domain.ErrValidation), errors.Is(err, domain.ErrInvalidCredentials):
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

	httpx.WriteJSON(w, http.StatusOK, AuthResponseFromDomain(response))
}
