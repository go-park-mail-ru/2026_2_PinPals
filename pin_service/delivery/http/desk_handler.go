package http

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"

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
	service DeskService
	baseURL string
}

func NewDeskHandler(service DeskService, baseURL string) *DeskHandler {
	return &DeskHandler{service: service, baseURL: baseURL}
}

type CreateDeskRequest struct {
	Name         string  `json:"name"`
	Description  *string `json:"description"`
	EntryStatus  int16   `json:"entry_status"`
	AvatarImgURL *string `json:"avatar_img_url"`
}

func (r CreateDeskRequest) ToDomain() domain.CreateDeskInput {
	return domain.CreateDeskInput{Name: r.Name, Description: r.Description, EntryStatus: r.EntryStatus, AvatarImgURL: r.AvatarImgURL}
}

type UpdateDeskRequest struct {
	Name         *string         `json:"name"`
	Description  json.RawMessage `json:"description"`
	EntryStatus  *int16          `json:"entry_status"`
	AvatarImgURL json.RawMessage `json:"avatar_img_url"`
}

func (r UpdateDeskRequest) ToDomain() (domain.UpdateDeskInput, error) {
	input := domain.UpdateDeskInput{Name: r.Name, EntryStatus: r.EntryStatus}
	if len(r.Description) > 0 {
		input.DescriptionSet = true
		v, err := decodeNullableString(r.Description)
		if err != nil {
			return input, err
		}
		input.Description = v
	}
	if len(r.AvatarImgURL) > 0 {
		input.AvatarImgURLSet = true
		v, err := decodeNullableString(r.AvatarImgURL)
		if err != nil {
			return input, err
		}
		input.AvatarImgURL = v
	}
	return input, nil
}

func decodeNullableString(raw json.RawMessage) (*string, error) {
	raw = bytes.TrimSpace(raw)
	if bytes.Equal(raw, []byte("null")) {
		return nil, nil
	}
	var value string
	if err := json.Unmarshal(raw, &value); err != nil {
		return nil, err
	}
	return &value, nil
}

type DeskResponse struct {
	ID           int           `json:"desk_id"`
	CreatorID    int           `json:"creator_id"`
	Name         string        `json:"name"`
	Description  *string       `json:"description,omitempty"`
	AvatarImgURL *string       `json:"avatar_img_url,omitempty"`
	EntryStatus  int16         `json:"entry_status"`
	CreatedAt    time.Time     `json:"created_at"`
	Pins         []PinResponse `json:"pins,omitempty"`
}

func (h *DeskHandler) response(d domain.Desk, pins []domain.Pin) DeskResponse {
	r := DeskResponse{ID: d.ID, CreatorID: d.CreatorID, Name: d.Name, Description: d.Description, AvatarImgURL: d.AvatarImgURL, EntryStatus: d.EntryStatus, CreatedAt: d.CreatedAt}
	if r.AvatarImgURL != nil && h.baseURL != "" {
		v := h.baseURL + "/images/" + url.PathEscape(*r.AvatarImgURL)
		r.AvatarImgURL = &v
	}
	if pins != nil {
		r.Pins = make([]PinResponse, len(pins))
		for i, p := range pins {
			r.Pins[i] = PinResponseFromDomain(p, h.baseURL)
		}
	}
	return r
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
	uid, ok := middleware.UserIDFromContext(r.Context())
	if !ok {
		httpx.WriteError(w, 401, "unauthorized")
		return
	}
	var req CreateDeskRequest
	if err := httpx.DecodeJSON(w, r, &req); err != nil {
		return
	}
	desk, err := h.service.Create(r.Context(), uid, req.ToDomain())
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
	uid, ok := middleware.UserIDFromContext(r.Context())
	if !ok {
		httpx.WriteError(w, 401, "unauthorized")
		return
	}
	desks, err := h.service.ListOwn(r.Context(), uid)
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
	if err != nil {
		httpx.WriteError(w, 400, "invalid user id")
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
	if err != nil {
		httpx.WriteError(w, 400, "invalid desk id")
		return
	}
	uid, _ := middleware.UserIDFromContext(r.Context())
	result, err := h.service.GetWithPins(r.Context(), uid, deskID)
	if err != nil {
		h.writeError(w, r, err)
		return
	}
	httpx.WriteJSON(w, 200, h.response(result.Desk, result.Pins))
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
	uid, ok := middleware.UserIDFromContext(r.Context())
	if !ok {
		httpx.WriteError(w, 401, "unauthorized")
		return
	}
	deskID, err := strconv.Atoi(r.PathValue("deskID"))
	if err != nil {
		httpx.WriteError(w, 400, "invalid desk id")
		return
	}
	var req UpdateDeskRequest
	if err := httpx.DecodeJSON(w, r, &req); err != nil {
		return
	}
	input, err := req.ToDomain()
	if err != nil {
		httpx.WriteError(w, http.StatusBadRequest, "invalid desk data")
		return
	}
	desk, err := h.service.Update(r.Context(), uid, deskID, input)
	if err != nil {
		h.writeError(w, r, err)
		return
	}
	httpx.WriteJSON(w, 200, h.response(desk, nil))
}

// Delete godoc
// @Summary Удалить доску
// @Tags Desks
// @Param deskID path int true "Идентификатор доски"
// @Success 204
// @Security CookieAuth
// @Router /desks/{deskID} [delete]
func (h *DeskHandler) Delete(w http.ResponseWriter, r *http.Request) {
	uid, ok := middleware.UserIDFromContext(r.Context())
	if !ok {
		httpx.WriteError(w, 401, "unauthorized")
		return
	}
	deskID, err := strconv.Atoi(r.PathValue("deskID"))
	if err != nil {
		httpx.WriteError(w, 400, "invalid desk id")
		return
	}
	if err := h.service.Delete(r.Context(), uid, deskID); err != nil {
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
func (h *DeskHandler) AddPin(w http.ResponseWriter, r *http.Request) { h.changePin(w, r, true) }

// RemovePin godoc
// @Summary Удалить пин с доски
// @Tags Desks
// @Param deskID path int true "Идентификатор доски"
// @Param pinID path int true "Идентификатор пина"
// @Success 204
// @Security CookieAuth
// @Router /desks/{deskID}/pins/{pinID} [delete]
func (h *DeskHandler) RemovePin(w http.ResponseWriter, r *http.Request) { h.changePin(w, r, false) }
func (h *DeskHandler) changePin(w http.ResponseWriter, r *http.Request, add bool) {
	uid, ok := middleware.UserIDFromContext(r.Context())
	if !ok {
		httpx.WriteError(w, 401, "unauthorized")
		return
	}
	deskID, e1 := strconv.Atoi(r.PathValue("deskID"))
	pinID, e2 := strconv.Atoi(r.PathValue("pinID"))
	if e1 != nil || e2 != nil {
		httpx.WriteError(w, 400, "invalid ids")
		return
	}
	var err error
	if add {
		err = h.service.AddPin(r.Context(), uid, deskID, pinID)
	} else {
		err = h.service.RemovePin(r.Context(), uid, deskID, pinID)
	}
	if err != nil {
		h.writeError(w, r, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}
func (h *DeskHandler) writeDesks(w http.ResponseWriter, desks []domain.Desk) {
	out := make([]DeskResponse, len(desks))
	for i, d := range desks {
		out[i] = h.response(d, nil)
	}
	httpx.WriteJSON(w, http.StatusOK, map[string]any{"desks": out})
}
func (h *DeskHandler) writeError(w http.ResponseWriter, r *http.Request, err error) {
	switch {
	case errors.Is(err, domain.ErrValidation):
		httpx.WriteError(w, 400, "invalid desk data")
	case errors.Is(err, domain.ErrDeskNotFound):
		httpx.WriteError(w, 404, "desk not found")
	case errors.Is(err, domain.ErrDeskForbidden):
		httpx.WriteError(w, 403, "desk access denied")
	case errors.Is(err, domain.ErrPinNotFound):
		httpx.WriteError(w, 404, "pin not found")
	default:
		middleware.LoggerFromContext(r.Context()).ErrorContext(r.Context(), "desk request failed", "error", err, "path", strings.TrimSpace(r.URL.Path))
		httpx.WriteError(w, 500, "desk request failed")
	}
}
