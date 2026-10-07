package pins

import (
	"context"
	"errors"
	"log/slog"
	"net/http"
	"net/url"
	"strconv"

	"2026_2_PinPals/internal/httpx"
	"2026_2_PinPals/internal/middleware"
	"2026_2_PinPals/internal/model"
)

type PinService interface {
	Create(
		ctx context.Context,
		creatorID int,
		input model.CreatePinInput,
	) (model.Pin, error)

	List(
		ctx context.Context,
		limit int,
		cursor *Cursor,
	) (Page, error)

	GetByID(
		ctx context.Context,
		id int,
	) (model.Pin, error)
}

type Handler struct {
	service PinService
	logger  *slog.Logger
	baseURL string
}

func NewHandler(service PinService, logger *slog.Logger, baseURL string) *Handler {
	return &Handler{
		service: service,
		logger:  logger,
		baseURL: baseURL,
	}
}

func (h *Handler) List(w http.ResponseWriter, r *http.Request) {
	type ListInput struct {
		Limit  *int    `json:"limit"`
		Cursor *Cursor `json:"cursor"`
	}

	var input ListInput
	if err := httpx.DecodeJSON(w, r, &input); err != nil {
		return
	}

	limit := 20
	if input.Limit != nil {
		limit = *input.Limit
	}

	cursor := input.Cursor
	if cursor != nil {
		if cursor.ID < 1 || cursor.CreatedAt.IsZero() {
			httpx.WriteError(w, http.StatusBadRequest, "invalid cursor")
			return
		}
	}

	page, err := h.service.List(r.Context(), limit, cursor)
	if err != nil {
		if errors.Is(err, ErrValidation) {
			httpx.WriteError(w, http.StatusBadRequest, "invalid pagination")
			return
		}

		h.logger.ErrorContext(
			r.Context(),
			"failed to get pins",
			"error", err,
		)

		httpx.WriteError(
			w,
			http.StatusInternalServerError,
			"failed to get pins",
		)
		return
	}

	pins := make([]model.Pin, len(page.Pins))

	for i, pin := range page.Pins {
		pins[i] = h.withImageURL(pin)
	}

	response := struct {
		Pins       []model.Pin `json:"pins"`
		NextCursor *Cursor     `json:"next_cursor,omitempty"`
	}{
		Pins:       pins,
		NextCursor: page.NextCursor,
	}

	httpx.WriteJSON(w, http.StatusOK, response)
}

func (h *Handler) GetByID(w http.ResponseWriter, r *http.Request) {
	id, err := strconv.Atoi(r.PathValue("pinID"))
	if err != nil {
		httpx.WriteError(w, http.StatusBadRequest, "invalid pin id")
		return
	}

	pin, err := h.service.GetByID(r.Context(), id)
	if err != nil {
		switch {
		case errors.Is(err, ErrValidation):
			httpx.WriteError(w, http.StatusBadRequest, "invalid pin id")

		case errors.Is(err, ErrPinNotFound):
			httpx.WriteError(w, http.StatusNotFound, "pin not found")

		default:
			h.logger.ErrorContext(
				r.Context(),
				"failed to get pin",
				"pin_id", id,
				"error", err,
			)

			httpx.WriteError(
				w,
				http.StatusInternalServerError,
				"failed to get pin",
			)
		}
		return
	}

	pin = h.withImageURL(pin)

	httpx.WriteJSON(w, http.StatusOK, pin)
}

func (h *Handler) Create(w http.ResponseWriter, r *http.Request) {
	userID, ok := middleware.UserIDFromContext(r.Context())
	if !ok {
		httpx.WriteError(w, http.StatusUnauthorized, "unauthorized")
		return
	}

	var input model.CreatePinInput
	if err := httpx.DecodeJSON(w, r, &input); err != nil {
		return
	}

	pin, err := h.service.Create(r.Context(), userID, input)
	if err != nil {
		if errors.Is(err, ErrValidation) {
			httpx.WriteError(w, http.StatusBadRequest, "invalid pin data")
			return
		}

		h.logger.ErrorContext(
			r.Context(),
			"failed to create pin",
			"user_id", userID,
			"error", err,
		)

		httpx.WriteError(
			w,
			http.StatusInternalServerError,
			"failed to create pin",
		)
		return
	}

	pin = h.withImageURL(pin)

	httpx.WriteJSON(w, http.StatusCreated, pin)
}

func (h *Handler) withImageURL(pin model.Pin) model.Pin {
	if h.baseURL == "" {
		return pin
	}

	pin.ImageURL = h.baseURL + "/images/" + url.PathEscape(pin.ImageURL)
	return pin
}
