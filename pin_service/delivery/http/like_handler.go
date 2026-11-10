package http

//go:generate go run go.uber.org/mock/mockgen@v0.6.0 -source=like_handler.go -destination=mocks/mock_like_service.go -package=mocks

import (
	"context"
	"errors"
	"net/http"
	"strconv"

	"2026_2_PinPals/pin_service/domain"
	"2026_2_PinPals/pkg/httpx"
	"2026_2_PinPals/pkg/middleware"
)

type LikeService interface {
	Add(context.Context, int, int) error
	Remove(context.Context, int, int) error
	Stats(context.Context, int, int) (domain.LikeStats, error)
}

type LikeHandler struct {
	service LikeService
}

func NewLikeHandler(service LikeService) *LikeHandler {
	return &LikeHandler{service: service}
}

// Add godoc
// @Summary Поставить лайк пину
// @Tags Likes
// @Produce json
// @Param pinID path int true "Идентификатор пина"
// @Success 200 {object} domain.LikeStats
// @Security CookieAuth
// @Router /pins/{pinID}/like [post]
func (h *LikeHandler) Add(w http.ResponseWriter, r *http.Request) {
	userID, ok := middleware.UserIDFromContext(r.Context())
	if !ok {
		httpx.WriteError(w, http.StatusUnauthorized, "Не авторизован")
		return
	}
	pinID, err := strconv.Atoi(r.PathValue("pinID"))
	if err != nil || pinID < 1 {
		httpx.WriteError(w, http.StatusBadRequest, "Некорректный идентификатор пина")
		return
	}
	if err := h.service.Add(r.Context(), userID, pinID); err != nil {
		h.writeError(w, r, err)
		return
	}

	stats, err := h.service.Stats(r.Context(), userID, pinID)
	if err != nil {
		h.writeError(w, r, err)
		return
	}
	httpx.WriteJSON(w, http.StatusOK, stats)
}

// Remove godoc
// @Summary Убрать лайк с пина
// @Tags Likes
// @Produce json
// @Param pinID path int true "Идентификатор пина"
// @Success 200 {object} domain.LikeStats
// @Security CookieAuth
// @Router /pins/{pinID}/like [delete]
func (h *LikeHandler) Remove(w http.ResponseWriter, r *http.Request) {
	userID, ok := middleware.UserIDFromContext(r.Context())
	if !ok {
		httpx.WriteError(w, http.StatusUnauthorized, "Не авторизован")
		return
	}
	pinID, err := strconv.Atoi(r.PathValue("pinID"))
	if err != nil || pinID < 1 {
		httpx.WriteError(w, http.StatusBadRequest, "Некорректный идентификатор пина")
		return
	}
	if err := h.service.Remove(r.Context(), userID, pinID); err != nil {
		h.writeError(w, r, err)
		return
	}

	stats, err := h.service.Stats(r.Context(), userID, pinID)
	if err != nil {
		h.writeError(w, r, err)
		return
	}
	httpx.WriteJSON(w, http.StatusOK, stats)
}

// GetStats godoc
// @Summary Получить количество лайков и признак лайка текущего пользователя
// @Tags Likes
// @Produce json
// @Param pinID path int true "Идентификатор пина"
// @Success 200 {object} domain.LikeStats
// @Router /pins/{pinID}/likes [get]
func (h *LikeHandler) GetStats(w http.ResponseWriter, r *http.Request) {
	pinID, err := strconv.Atoi(r.PathValue("pinID"))
	if err != nil || pinID < 1 {
		httpx.WriteError(w, http.StatusBadRequest, "Некорректный идентификатор пина")
		return
	}
	userID, _ := middleware.UserIDFromContext(r.Context())
	stats, err := h.service.Stats(r.Context(), userID, pinID)
	if err != nil {
		h.writeError(w, r, err)
		return
	}
	httpx.WriteJSON(w, http.StatusOK, stats)
}

func (h *LikeHandler) writeError(w http.ResponseWriter, r *http.Request, err error) {
	switch {
	case errors.Is(err, domain.ErrValidation):
		httpx.WriteError(w, http.StatusBadRequest, "Некорректный идентификатор пина")
	case errors.Is(err, domain.ErrPinNotFound):
		httpx.WriteError(w, http.StatusNotFound, "Пин не найден")
	default:
		middleware.LoggerFromContext(r.Context()).ErrorContext(r.Context(), "pin like request failed", "error", err)
		httpx.WriteError(w, http.StatusInternalServerError, "Не удалось обработать лайк")
	}
}
