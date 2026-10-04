package auth

import (
	"2026_2_PinPals/internal/httpx"
	"2026_2_PinPals/internal/model"
	"errors"
	"log/slog"
	"net/http"
)

type TokenIssuer func(userID int) (string, error)

type Handler struct {
	service     *Service
	tokenIssuer TokenIssuer
	logger      *slog.Logger
}

func NewHandler(service *Service, tokenIssuer TokenIssuer, logger *slog.Logger) *Handler {
	return &Handler{service: service, tokenIssuer: tokenIssuer, logger: logger}
}

func (h *Handler) Register(w http.ResponseWriter, r *http.Request) {
	var input model.RegisterInput
	if err := httpx.DecodeJSON(w, r, &input); err != nil {
		return
	}

	user, err := h.service.Register(r.Context(), input)
	if err != nil {
		switch {
		case errors.Is(err, ErrValidation):
			httpx.WriteError(w, http.StatusBadRequest, "invalid registration data")
		case errors.Is(err, ErrUserTagExists):
			httpx.WriteError(w, http.StatusConflict, "user tag already exists")
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

	httpx.WriteJSON(w, http.StatusCreated, user)
}

func (h *Handler) Login(w http.ResponseWriter, r *http.Request) {
	var input model.LoginInput
	if err := httpx.DecodeJSON(w, r, &input); err != nil {
		return
	}

	response, err := h.service.Login(r.Context(), input, h.tokenIssuer)
	if err != nil {
		switch {
		case errors.Is(err, ErrValidation), errors.Is(err, ErrInvalidCredentials):
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

	httpx.WriteJSON(w, http.StatusOK, response)
}
