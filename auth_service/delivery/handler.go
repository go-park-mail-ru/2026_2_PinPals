package delivery

import (
	"context"
	"errors"
	"log/slog"
	"net/http"

	"2026_2_PinPals/auth_service/domain"
	"2026_2_PinPals/pkg/httpx"
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
	logger      *slog.Logger
}

func NewHandler(service AuthService, tokenIssuer TokenIssuer, logger *slog.Logger) *Handler {
	return &Handler{service: service, tokenIssuer: tokenIssuer, logger: logger}
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
					"error": "conflict",
					"fields": map[string]string{
						"user_tag": "already_exists",
					},
				},
			)

		default:
			h.logger.ErrorContext(
				r.Context(),
				"failed to register user",
				"error", err,
			)

			httpx.WriteError(
				w,
				http.StatusInternalServerError,
				"failed to register user",
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
			httpx.WriteError(w, http.StatusUnauthorized, "invalid credentials")
		default:
			h.logger.ErrorContext(
				r.Context(),
				"failed to authenticate",
				"error", err,
			)

			httpx.WriteError(
				w,
				http.StatusInternalServerError,
				"failed to authenticate",
			)
		}
		return
	}

	httpx.WriteJSON(w, http.StatusOK, AuthResponseFromDomain(response))
}
