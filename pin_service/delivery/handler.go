package delivery

import (
	"context"
	"errors"
	"log/slog"
	"net/http"
	"net/url"
	"strconv"

	"2026_2_PinPals/internal/httpx"
	"2026_2_PinPals/internal/middleware"
	"2026_2_PinPals/pin_service/domain"
)

type PinService interface {
	Create(
		ctx context.Context,
		creatorID int,
		input domain.CreatePinInput,
	) (domain.Pin, error)

	List(
		ctx context.Context,
		limit int,
		cursor *domain.Cursor,
	) (domain.Page, error)

	GetByID(
		ctx context.Context,
		id int,
	) (domain.Pin, error)
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
		Limit  *int           `json:"limit"`
		Cursor *domain.Cursor `json:"cursor"`
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
		if errors.Is(err, domain.ErrValidation) {
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

	pins := make([]domain.Pin, len(page.Pins))

	for i, pin := range page.Pins {
		pins[i] = h.withImageURL(pin)
	}

	response := struct {
		Pins       []domain.Pin   `json:"pins"`
		NextCursor *domain.Cursor `json:"next_cursor,omitempty"`
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
		case errors.Is(err, domain.ErrValidation):
			httpx.WriteError(w, http.StatusBadRequest, "invalid pin id")

		case errors.Is(err, domain.ErrPinNotFound):
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

	var input domain.CreatePinInput
	if err := httpx.DecodeJSON(w, r, &input); err != nil {
		return
	}

	pin, err := h.service.Create(r.Context(), userID, input)
	if err != nil {
		if errors.Is(err, domain.ErrValidation) {
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

func (h *Handler) withImageURL(pin domain.Pin) domain.Pin {
	if h.baseURL == "" {
		return pin
	}

	pin.ImageURL = h.baseURL + "/images/" + url.PathEscape(pin.ImageURL)
	return pin
}
