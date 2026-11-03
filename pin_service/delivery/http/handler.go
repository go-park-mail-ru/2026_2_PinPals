package http

import (
	"context"
	"errors"
	"log/slog"
	"net/http"
	"strconv"
	"strings"

	"github.com/go-playground/validator/v10"

	"2026_2_PinPals/pin_service/domain"
	"2026_2_PinPals/pkg/httpx"
	"2026_2_PinPals/pkg/middleware"
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
	service          PinService
	logger           *slog.Logger
	baseURL          string
	requestValidator *validator.Validate
}

func NewHandler(
	service PinService,
	logger *slog.Logger,
	baseURL string,
	requestValidator *validator.Validate,
) *Handler {
	return &Handler{
		service:          service,
		logger:           logger,
		baseURL:          baseURL,
		requestValidator: requestValidator,
	}
}

func (h *Handler) List(w http.ResponseWriter, r *http.Request) {
	var req ListRequest
	if err := httpx.DecodeJSON(w, r, &req); err != nil {
		return
	}

	if err := h.requestValidator.Struct(req); err != nil {
		httpx.WriteError(w, http.StatusBadRequest, "invalid pagination")
		return
	}

	limit, cursor := req.ToDomain()

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

	httpx.WriteJSON(w, http.StatusOK, ListResponseFromDomain(page, h.baseURL))
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

	httpx.WriteJSON(w, http.StatusOK, PinResponseFromDomain(pin, h.baseURL))
}

func (h *Handler) Create(w http.ResponseWriter, r *http.Request) {
	userID, ok := middleware.UserIDFromContext(r.Context())
	if !ok {
		httpx.WriteError(w, http.StatusUnauthorized, "unauthorized")
		return
	}

	var req CreatePinRequest
	if err := httpx.DecodeJSON(w, r, &req); err != nil {
		return
	}

	req.Name = strings.TrimSpace(req.Name)
	req.ImageURL = strings.TrimSpace(req.ImageURL)

	if err := h.requestValidator.Struct(req); err != nil {
		httpx.WriteError(
			w,
			http.StatusBadRequest,
			"invalid pin data",
		)
		return
	}

	input := req.ToDomain()

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

	httpx.WriteJSON(w, http.StatusCreated, PinResponseFromDomain(pin, h.baseURL))
}
