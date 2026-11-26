package http

import (
	"context"
	"errors"
	"net/http"
	"strconv"

	"2026_2_PinPals/pkg/httpx"
	"2026_2_PinPals/pkg/middleware"
	"2026_2_PinPals/user_service/domain"
)

type SubscriptionService interface {
	Subscribe(ctx context.Context, subscriberID, subscribeeID int) error
	Unsubscribe(ctx context.Context, subscriberID, subscribeeID int) error
}

type SubscriptionHandler struct{ service SubscriptionService }

func NewSubscriptionHandler(service SubscriptionService) *SubscriptionHandler {
	return &SubscriptionHandler{service: service}
}

// Subscribe godoc
// @Summary Подписаться на пользователя
// @Tags Subscriptions
// @Param userID path int true "Идентификатор пользователя, на которого подписываются"
// @Success 204
// @Failure 400 {object} map[string]interface{}
// @Failure 401 {object} map[string]interface{}
// @Failure 404 {object} map[string]interface{}
// @Security CookieAuth
// @Router /users/{userID}/subscription [post]
func (h *SubscriptionHandler) Subscribe(w http.ResponseWriter, r *http.Request) {
	userID, targetID, ok := h.parseParticipants(w, r)
	if !ok {
		return
	}
	if err := h.service.Subscribe(r.Context(), userID, targetID); err != nil {
		h.writeError(w, r, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

// Unsubscribe godoc
// @Summary Отписаться от пользователя
// @Tags Subscriptions
// @Param userID path int true "Идентификатор пользователя, от которого отписываются"
// @Success 204
// @Failure 400 {object} map[string]interface{}
// @Failure 401 {object} map[string]interface{}
// @Failure 404 {object} map[string]interface{}
// @Security CookieAuth
// @Router /users/{userID}/subscription [delete]
func (h *SubscriptionHandler) Unsubscribe(w http.ResponseWriter, r *http.Request) {
	userID, targetID, ok := h.parseParticipants(w, r)
	if !ok {
		return
	}
	if err := h.service.Unsubscribe(r.Context(), userID, targetID); err != nil {
		h.writeError(w, r, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func (h *SubscriptionHandler) parseParticipants(w http.ResponseWriter, r *http.Request) (int, int, bool) {
	userID, ok := middleware.UserIDFromContext(r.Context())
	if !ok {
		httpx.WriteError(w, http.StatusUnauthorized, "необходима авторизация")
		return 0, 0, false
	}
	targetID, err := strconv.Atoi(r.PathValue("userID"))
	if err != nil || targetID < 1 {
		httpx.WriteError(w, http.StatusBadRequest, "некорректный идентификатор пользователя")
		return 0, 0, false
	}
	return userID, targetID, true
}

func (h *SubscriptionHandler) writeError(w http.ResponseWriter, r *http.Request, err error) {
	switch {
	case errors.Is(err, domain.ErrValidation):
		httpx.WriteError(w, http.StatusBadRequest, "некорректные данные подписки")
	case errors.Is(err, domain.ErrUserNotFound):
		httpx.WriteError(w, http.StatusNotFound, "пользователь не найден")
	case errors.Is(err, domain.ErrSubscriptionNotFound):
		httpx.WriteError(w, http.StatusNotFound, "подписка не найдена")
	default:
		middleware.LoggerFromContext(r.Context()).ErrorContext(r.Context(), "не удалось изменить подписку", "error", err, "path", r.URL.Path)
		httpx.WriteError(w, http.StatusInternalServerError, "не удалось изменить подписку")
	}
}
