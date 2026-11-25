package http

import (
	"context"
	"errors"
	"net/http"

	"2026_2_PinPals/pin_service/domain"
	"2026_2_PinPals/pkg/httpx"
	"2026_2_PinPals/pkg/middleware"
)

type SearchService interface {
	SearchPins(ctx context.Context, query string, limit, offset int) ([]domain.Pin, error)
	SearchDesks(ctx context.Context, query string, limit, offset int) ([]domain.Desk, error)
}

type SearchHandler struct {
	service SearchService
	baseURL string
}

func NewSearchHandler(service SearchService, baseURL string) *SearchHandler {
	return &SearchHandler{service: service, baseURL: baseURL}
}

// SearchPins godoc
// @Summary Найти пины по названию
// @Tags Search
// @Produce json
// @Param q query string true "Поисковый запрос (название пина), до 100 символов"
// @Param limit query int false "Количество результатов (1–100, по умолчанию 20)"
// @Param offset query int false "Смещение от начала выдачи (по умолчанию 0)"
// @Success 200 {object} SearchPinsResponse
// @Failure 400 {object} map[string]interface{}
// @Router /pins/search [get]
func (h *SearchHandler) SearchPins(w http.ResponseWriter, r *http.Request) {
	req, err := NewSearchRequest(r.URL.Query())
	if err != nil {
		h.writeError(w, r, err)
		return
	}
	pins, err := h.service.SearchPins(r.Context(), req.Query, req.Limit, req.Offset)
	if err != nil {
		h.writeError(w, r, err)
		return
	}
	httpx.WriteJSON(w, http.StatusOK, SearchPinsResponseFromDomain(pins, h.baseURL))
}

// SearchDesks godoc
// @Summary Найти публичные доски по названию
// @Tags Search
// @Produce json
// @Param q query string true "Поисковый запрос (название доски), до 100 символов"
// @Param limit query int false "Количество результатов (1–100, по умолчанию 20)"
// @Param offset query int false "Смещение от начала выдачи (по умолчанию 0)"
// @Success 200 {object} SearchDesksResponse
// @Failure 400 {object} map[string]interface{}
// @Router /desks/search [get]
func (h *SearchHandler) SearchDesks(w http.ResponseWriter, r *http.Request) {
	req, err := NewSearchRequest(r.URL.Query())
	if err != nil {
		h.writeError(w, r, err)
		return
	}
	desks, err := h.service.SearchDesks(r.Context(), req.Query, req.Limit, req.Offset)
	if err != nil {
		h.writeError(w, r, err)
		return
	}
	httpx.WriteJSON(w, http.StatusOK, SearchDesksResponseFromDomain(desks, h.baseURL))
}

func (h *SearchHandler) writeError(w http.ResponseWriter, r *http.Request, err error) {
	switch {
	case errors.Is(err, domain.ErrValidation):
		httpx.WriteError(w, http.StatusBadRequest, "invalid search parameters")
	default:
		middleware.LoggerFromContext(r.Context()).ErrorContext(r.Context(), "search request failed", "error", err, "path", r.URL.Path)
		httpx.WriteError(w, http.StatusInternalServerError, "search request failed")
	}
}
