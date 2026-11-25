package http

import (
	"context"
	"errors"
	"net/http"

	"2026_2_PinPals/pkg/httpx"
	"2026_2_PinPals/pkg/middleware"
	"2026_2_PinPals/user_service/domain"
)

type SearchService interface {
	SearchUsers(ctx context.Context, query string, limit, offset int) ([]domain.User, error)
}

type SearchHandler struct {
	service SearchService
	baseURL string
}

func NewSearchHandler(service SearchService, baseURL string) *SearchHandler {
	return &SearchHandler{service: service, baseURL: baseURL}
}

// SearchUsers godoc
// @Summary Найти пользователей по тегу
// @Tags Search
// @Produce json
// @Param q query string true "Поисковый запрос (тег пользователя), до 100 символов"
// @Param limit query int false "Количество результатов (1–100, по умолчанию 20)"
// @Param offset query int false "Смещение от начала выдачи (по умолчанию 0)"
// @Success 200 {object} SearchUsersResponse
// @Failure 400 {object} map[string]interface{}
// @Failure 401 {object} map[string]interface{}
// @Security CookieAuth
// @Router /users/search [get]
func (h *SearchHandler) SearchUsers(w http.ResponseWriter, r *http.Request) {
	req, err := NewSearchRequest(r.URL.Query())
	if err != nil {
		h.writeError(w, r, err)
		return
	}
	users, err := h.service.SearchUsers(r.Context(), req.Query, req.Limit, req.Offset)
	if err != nil {
		h.writeError(w, r, err)
		return
	}
	httpx.WriteJSON(w, http.StatusOK, SearchUsersResponseFromDomain(users, h.baseURL))
}

func (h *SearchHandler) writeError(w http.ResponseWriter, r *http.Request, err error) {
	switch {
	case errors.Is(err, domain.ErrValidation):
		httpx.WriteError(w, http.StatusBadRequest, "некорректные параметры поиска")
	default:
		middleware.LoggerFromContext(r.Context()).ErrorContext(r.Context(), "не удалось выполнить поиск пользователей", "error", err, "path", r.URL.Path)
		httpx.WriteError(w, http.StatusInternalServerError, "не удалось выполнить поиск пользователей")
	}
}
