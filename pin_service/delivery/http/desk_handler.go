package http

//go:generate go run go.uber.org/mock/mockgen@v0.6.0 -source=desk_handler.go -destination=mocks/mock_desk_service.go -package=mocks

import (
	"context"
	"errors"
	"net/http"
	"strconv"
	"strings"

	"github.com/go-playground/validator/v10"

	"2026_2_PinPals/pin_service/domain"
	"2026_2_PinPals/pkg/httpx"
	"2026_2_PinPals/pkg/middleware"
)

type DeskService interface {
	Create(context.Context, int, domain.CreateDeskInput) (domain.Desk, error)
	ListOwn(context.Context, int) ([]domain.Desk, error)
	ListPublic(context.Context, int, int) ([]domain.Desk, error)
	GetWithPins(context.Context, int, int) (domain.DeskWithPins, error)
	Update(context.Context, int, int, domain.UpdateDeskInput) (domain.Desk, error)
	Delete(context.Context, int, int) error
	AddPin(context.Context, int, int, int) error
	RemovePin(context.Context, int, int, int) error
}

type DeskHandler struct {
	service          DeskService
	baseURL          string
	requestValidator *validator.Validate
}

func NewDeskHandler(service DeskService, baseURL string, requestValidator *validator.Validate) *DeskHandler {
	if requestValidator == nil {
		requestValidator = validator.New()
	}
	return &DeskHandler{
		service:          service,
		baseURL:          baseURL,
		requestValidator: requestValidator,
	}
}

func (h *DeskHandler) response(desk domain.Desk, pins []domain.Pin) DeskResponse {
	return DeskResponseFromDomain(desk, pins, h.baseURL)
}

// Create godoc
// @Summary Создать доску
// @Tags Desks
// @Accept json
// @Produce json
// @Param request body CreateDeskRequest true "Данные доски"
// @Success 201 {object} DeskResponse
// @Security CookieAuth
// @Router /desks [post]
func (h *DeskHandler) Create(w http.ResponseWriter, r *http.Request) {
	userID, ok := middleware.UserIDFromContext(r.Context())
	if !ok {
		httpx.WriteError(w, http.StatusUnauthorized, "Не авторизован")
		return
	}

	var request CreateDeskRequest
	if err := httpx.DecodeJSON(w, r, &request); err != nil {
		return
	}
	if err := h.requestValidator.Struct(request); err != nil {
		httpx.WriteError(w, http.StatusBadRequest, "Некорректные данные доски")
		return
	}

	desk, err := h.service.Create(r.Context(), userID, request.ToDomain())
	if err != nil {
		h.writeError(w, r, err)
		return
	}
	httpx.WriteJSON(w, http.StatusCreated, h.response(desk, nil))
}

// ListOwn godoc
// @Summary Получить свои доски
// @Tags Desks
// @Produce json
// @Success 200 {object} map[string]interface{}
// @Security CookieAuth
// @Router /desks/me [get]
func (h *DeskHandler) ListOwn(w http.ResponseWriter, r *http.Request) {
	userID, ok := middleware.UserIDFromContext(r.Context())
	if !ok {
		httpx.WriteError(w, http.StatusUnauthorized, "Не авторизован")
		return
	}

	desks, err := h.service.ListOwn(r.Context(), userID)
	if err != nil {
		h.writeError(w, r, err)
		return
	}
	h.writeDesks(w, desks)
}

// ListPublic godoc
// @Summary Получить публичные доски пользователя
// @Tags Desks
// @Produce json
// @Param userID path int true "Идентификатор пользователя"
// @Success 200 {object} map[string]interface{}
// @Router /users/{userID}/desks [get]
func (h *DeskHandler) ListPublic(w http.ResponseWriter, r *http.Request) {
	ownerID, err := strconv.Atoi(r.PathValue("userID"))
	if err != nil || ownerID < 1 {
		httpx.WriteError(w, http.StatusBadRequest, "Некорректный идентификатор пользователя")
		return
	}

	desks, err := h.service.ListPublic(r.Context(), ownerID, 100)
	if err != nil {
		h.writeError(w, r, err)
		return
	}
	h.writeDesks(w, desks)
}

// GetByID godoc
// @Summary Получить доску с пинами
// @Tags Desks
// @Produce json
// @Param deskID path int true "Идентификатор доски"
// @Success 200 {object} DeskResponse
// @Failure 403 {object} map[string]interface{}
// @Failure 404 {object} map[string]interface{}
// @Router /desks/{deskID} [get]
func (h *DeskHandler) GetByID(w http.ResponseWriter, r *http.Request) {
	deskID, err := strconv.Atoi(r.PathValue("deskID"))
	if err != nil || deskID < 1 {
		httpx.WriteError(w, http.StatusBadRequest, "Некорректный идентификатор доски")
		return
	}
	userID, _ := middleware.UserIDFromContext(r.Context())

	result, err := h.service.GetWithPins(r.Context(), userID, deskID)
	if err != nil {
		h.writeError(w, r, err)
		return
	}
	httpx.WriteJSON(w, http.StatusOK, h.response(result.Desk, result.Pins))
}

// Update godoc
// @Summary Изменить доску
// @Tags Desks
// @Accept json
// @Produce json
// @Param deskID path int true "Идентификатор доски"
// @Param request body UpdateDeskRequest true "Изменения доски"
// @Success 200 {object} DeskResponse
// @Security CookieAuth
// @Router /desks/{deskID} [patch]
func (h *DeskHandler) Update(w http.ResponseWriter, r *http.Request) {
	userID, ok := middleware.UserIDFromContext(r.Context())
	if !ok {
		httpx.WriteError(w, http.StatusUnauthorized, "Не авторизован")
		return
	}
	deskID, err := strconv.Atoi(r.PathValue("deskID"))
	if err != nil || deskID < 1 {
		httpx.WriteError(w, http.StatusBadRequest, "Некорректный идентификатор доски")
		return
	}

	var request UpdateDeskRequest
	if err := httpx.DecodeJSON(w, r, &request); err != nil {
		return
	}
	if err := request.Validate(h.requestValidator); err != nil {
		httpx.WriteError(w, http.StatusBadRequest, "Некорректные данные доски")
		return
	}
	input, err := request.ToDomain()
	if err != nil {
		httpx.WriteError(w, http.StatusBadRequest, "Некорректные данные доски")
		return
	}

	desk, err := h.service.Update(r.Context(), userID, deskID, input)
	if err != nil {
		h.writeError(w, r, err)
		return
	}
	httpx.WriteJSON(w, http.StatusOK, h.response(desk, nil))
}

// Delete godoc
// @Summary Удалить доску
// @Tags Desks
// @Param deskID path int true "Идентификатор доски"
// @Success 204
// @Security CookieAuth
// @Router /desks/{deskID} [delete]
func (h *DeskHandler) Delete(w http.ResponseWriter, r *http.Request) {
	userID, ok := middleware.UserIDFromContext(r.Context())
	if !ok {
		httpx.WriteError(w, http.StatusUnauthorized, "Не авторизован")
		return
	}
	deskID, err := strconv.Atoi(r.PathValue("deskID"))
	if err != nil || deskID < 1 {
		httpx.WriteError(w, http.StatusBadRequest, "Некорректный идентификатор доски")
		return
	}

	if err := h.service.Delete(r.Context(), userID, deskID); err != nil {
		h.writeError(w, r, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

// AddPin godoc
// @Summary Добавить пин на доску
// @Tags Desks
// @Param deskID path int true "Идентификатор доски"
// @Param pinID path int true "Идентификатор пина"
// @Success 204
// @Security CookieAuth
// @Router /desks/{deskID}/pins/{pinID} [post]
func (h *DeskHandler) AddPin(w http.ResponseWriter, r *http.Request) {
	h.changePin(w, r, true)
}

// RemovePin godoc
// @Summary Удалить пин с доски
// @Tags Desks
// @Param deskID path int true "Идентификатор доски"
// @Param pinID path int true "Идентификатор пина"
// @Success 204
// @Security CookieAuth
// @Router /desks/{deskID}/pins/{pinID} [delete]
func (h *DeskHandler) RemovePin(w http.ResponseWriter, r *http.Request) {
	h.changePin(w, r, false)
}

func (h *DeskHandler) changePin(w http.ResponseWriter, r *http.Request, add bool) {
	userID, ok := middleware.UserIDFromContext(r.Context())
	if !ok {
		httpx.WriteError(w, http.StatusUnauthorized, "Не авторизован")
		return
	}
	deskID, deskErr := strconv.Atoi(r.PathValue("deskID"))
	pinID, pinErr := strconv.Atoi(r.PathValue("pinID"))
	if deskErr != nil || pinErr != nil || deskID < 1 || pinID < 1 {
		httpx.WriteError(w, http.StatusBadRequest, "Некорректные идентификаторы доски и пина")
		return
	}

	var err error
	if add {
		err = h.service.AddPin(r.Context(), userID, deskID, pinID)
	} else {
		err = h.service.RemovePin(r.Context(), userID, deskID, pinID)
	}
	if err != nil {
		h.writeError(w, r, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func (h *DeskHandler) writeDesks(w http.ResponseWriter, desks []domain.Desk) {
	out := make([]DeskResponse, len(desks))
	for i, desk := range desks {
		out[i] = h.response(desk, nil)
	}
	httpx.WriteJSON(w, http.StatusOK, map[string]any{"desks": out})
}

func (h *DeskHandler) writeError(w http.ResponseWriter, r *http.Request, err error) {
	switch {
	case errors.Is(err, domain.ErrValidation):
		httpx.WriteError(w, http.StatusBadRequest, "Некорректные данные доски")
	case errors.Is(err, domain.ErrDeskNotFound):
		httpx.WriteError(w, http.StatusNotFound, "Доска не найдена")
	case errors.Is(err, domain.ErrDeskForbidden):
		httpx.WriteError(w, http.StatusForbidden, "Нет доступа к доске")
	case errors.Is(err, domain.ErrPinNotFound):
		httpx.WriteError(w, http.StatusNotFound, "Пин не найден")
	default:
		middleware.LoggerFromContext(r.Context()).ErrorContext(r.Context(), "desk request failed", "error", err, "path", strings.TrimSpace(r.URL.Path))
		httpx.WriteError(w, http.StatusInternalServerError, "Не удалось выполнить запрос к доске")
	}
}
