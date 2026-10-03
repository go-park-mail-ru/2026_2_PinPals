package pins

import (
	"errors"
	"log/slog"
	"net/http"
	"strconv"

	"2026_2_PinPals/internal/httpx"
	"2026_2_PinPals/internal/middleware"
	"2026_2_PinPals/internal/model"
)

type Handler struct {
	service *Service
	logger  *slog.Logger
}

func NewHandler(service *Service, logger *slog.Logger) *Handler {
	return &Handler{
		service: service,
		logger:  logger,
	}
}

func (h *Handler) List(w http.ResponseWriter, r *http.Request) {
	limit := 20

	if raw := r.URL.Query().Get("limit"); raw != "" {
		parsedLimit, err := strconv.Atoi(raw)
		if err != nil {
			httpx.WriteError(w, http.StatusBadRequest, "invalid limit")
			return
		}

		limit = parsedLimit
	}

	var cursor *Cursor
	if raw := r.URL.Query().Get("cursor"); raw != "" {
		parsedCursor, err := decodeCursor(raw)
		if err != nil {
			httpx.WriteError(w, http.StatusBadRequest, "invalid cursor")
			return
		}

		cursor = parsedCursor
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

		httpx.WriteError(w, http.StatusInternalServerError, "failed to get pins")
		return
	}

	response := struct {
		Pins       []model.Pin `json:"pins"`
		NextCursor string      `json:"next_cursor,omitempty"`
	}{
		Pins: page.Pins,
	}

	if page.NextCursor != nil {
		encodedCursor, err := encodeCursor(*page.NextCursor)
		if err != nil {
			h.logger.ErrorContext(
				r.Context(),
				"failed to encode pins cursor",
				"error", err,
			)

			httpx.WriteError(
				w,
				http.StatusInternalServerError,
				"failed to encode cursor",
			)
			return
		}

		response.NextCursor = encodedCursor
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

	httpx.WriteJSON(w, http.StatusCreated, pin)
}
