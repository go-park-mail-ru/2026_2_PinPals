package http

import (
	"context"
	"errors"
	"net/http"

	"2026_2_PinPals/pkg/httpx"
	"2026_2_PinPals/pkg/middleware"
	"2026_2_PinPals/user_service/domain"
)

type UserService interface {
	GetProfile(ctx context.Context, userID int) (*domain.User, error)
	UpdateProfile(ctx context.Context, userID int, input domain.UpdateProfileInput) (*domain.User, error)
	DeleteProfile(ctx context.Context, userID int) error
}

// TokenRevoker отзывает JWT на сервере (реализуется middleware.TokenManager).
type TokenRevoker interface {
	Revoke(ctx context.Context, token string) error
}

type HandlerOptions struct {
	TokenRevoker  TokenRevoker
	SecureCookies bool
}

type Handler struct {
	service UserService
	options HandlerOptions
}

func NewHandler(service UserService, options ...HandlerOptions) *Handler {
	h := &Handler{service: service}
	if len(options) > 0 {
		h.options = options[0]
	}
	return h
}

// GetMe godoc
// @Summary Получить профиль текущего пользователя
// @Tags Users
// @Produce json
// @Success 200 {object} UserResponse
// @Security CookieAuth
// @Router /users/me [get]
func (h *Handler) GetMe(w http.ResponseWriter, r *http.Request) {
	userID, ok := middleware.UserIDFromContext(r.Context())
	if !ok {
		httpx.WriteError(w, http.StatusUnauthorized, "unauthorized")
		return
	}
	user, err := h.service.GetProfile(r.Context(), userID)
	if err != nil {
		h.writeError(w, r, err, "failed to get profile")
		return
	}
	httpx.WriteJSON(w, http.StatusOK, UserResponseFromDomain(*user))
}

// UpdateMe godoc
// @Summary Изменить профиль текущего пользователя
// @Tags Users
// @Accept json
// @Produce json
// @Param request body UpdateProfileRequest true "Поля профиля"
// @Success 200 {object} UserResponse
// @Security CookieAuth
// @Router /users/me [patch]
func (h *Handler) UpdateMe(w http.ResponseWriter, r *http.Request) {
	userID, ok := middleware.UserIDFromContext(r.Context())
	if !ok {
		httpx.WriteError(w, http.StatusUnauthorized, "unauthorized")
		return
	}
	var req UpdateProfileRequest
	if err := httpx.DecodeJSON(w, r, &req); err != nil {
		return
	}
	input, err := req.ToDomain()
	if err != nil {
		httpx.WriteError(w, http.StatusBadRequest, "invalid profile data")
		return
	}
	user, err := h.service.UpdateProfile(r.Context(), userID, input)
	if err != nil {
		h.writeError(w, r, err, "failed to update profile")
		return
	}
	httpx.WriteJSON(w, http.StatusOK, UserResponseFromDomain(*user))
}

// DeleteMe godoc
// @Summary Удалить профиль текущего пользователя
// @Tags Users
// @Success 204
// @Security CookieAuth
// @Router /users/me [delete]
func (h *Handler) DeleteMe(w http.ResponseWriter, r *http.Request) {
	userID, ok := middleware.UserIDFromContext(r.Context())
	if !ok {
		httpx.WriteError(w, http.StatusUnauthorized, "unauthorized")
		return
	}
	if err := h.service.DeleteProfile(r.Context(), userID); err != nil {
		h.writeError(w, r, err, "failed to delete profile")
		return
	}
	// Профиль удалён: отзываем текущий токен и очищаем cookie.
	// Токены остальных сессий отклоняются middleware по признаку удалённого пользователя.
	if h.options.TokenRevoker != nil {
		if token := middleware.TokenFromRequest(r); token != "" {
			if err := h.options.TokenRevoker.Revoke(r.Context(), token); err != nil {
				middleware.LoggerFromContext(r.Context()).ErrorContext(r.Context(), "failed to revoke token after profile deletion", "error", err)
			}
		}
	}
	middleware.ClearAuthCookies(w, h.options.SecureCookies)
	w.WriteHeader(http.StatusNoContent)
}

func (h *Handler) writeError(w http.ResponseWriter, r *http.Request, err error, fallback string) {
	switch {
	case errors.Is(err, domain.ErrValidation):
		httpx.WriteError(w, http.StatusBadRequest, "invalid profile data")
	case errors.Is(err, domain.ErrUserNotFound):
		httpx.WriteError(w, http.StatusNotFound, "user not found")
	case errors.Is(err, domain.ErrUserTagExists):
		httpx.WriteJSON(w, http.StatusConflict, map[string]any{"error": "conflict", "fields": map[string]string{"user_tag": "already_exists"}})
	default:
		middleware.LoggerFromContext(r.Context()).ErrorContext(r.Context(), fallback, "error", err, "path", r.URL.Path)
		httpx.WriteError(w, http.StatusInternalServerError, fallback)
	}
}
