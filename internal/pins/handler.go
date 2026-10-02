package pins

import (
	"encoding/json"
	"errors"
	"net/http"
	"strconv"

	"2026_2_PinPals/internal/middleware"
	"2026_2_PinPals/internal/model"
)

type Handler struct {
	service *Service
}

func NewHandler(service *Service) *Handler {
	return &Handler{service: service}
}

func (h *Handler) List(w http.ResponseWriter, r *http.Request) {
	limit := 20
	offset := 0
	var err error

	if raw := r.URL.Query().Get("limit"); raw != "" {
		limit, err = strconv.Atoi(raw)
		if err != nil {
			writeError(w, http.StatusBadRequest, "invalid limit")
			return
		}
	}
	if raw := r.URL.Query().Get("offset"); raw != "" {
		offset, err = strconv.Atoi(raw)
		if err != nil {
			writeError(w, http.StatusBadRequest, "invalid offset")
			return
		}
	}

	result, err := h.service.List(r.Context(), limit, offset)
	if err != nil {
		if errors.Is(err, ErrValidation) {
			writeError(w, http.StatusBadRequest, "invalid pagination")
			return
		}
		writeError(w, http.StatusInternalServerError, "failed to get pins")
		return
	}

	writeJSON(w, http.StatusOK, map[string]any{"pins": result})
}

func (h *Handler) GetByID(w http.ResponseWriter, r *http.Request) {
	id, err := strconv.Atoi(r.PathValue("pinID"))
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid pin id")
		return
	}

	pin, err := h.service.GetByID(r.Context(), id)
	if err != nil {
		switch {
		case errors.Is(err, ErrValidation):
			writeError(w, http.StatusBadRequest, "invalid pin id")
		case errors.Is(err, ErrPinNotFound):
			writeError(w, http.StatusNotFound, "pin not found")
		default:
			writeError(w, http.StatusInternalServerError, "failed to get pin")
		}
		return
	}

	writeJSON(w, http.StatusOK, pin)
}

func (h *Handler) Create(w http.ResponseWriter, r *http.Request) {
	userID, ok := middleware.UserIDFromContext(r.Context())
	if !ok {
		writeError(w, http.StatusUnauthorized, "unauthorized")
		return
	}

	var input model.CreatePinInput
	decoder := json.NewDecoder(r.Body)
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&input); err != nil {
		writeError(w, http.StatusBadRequest, "invalid json body")
		return
	}

	pin, err := h.service.Create(r.Context(), userID, input)
	if err != nil {
		if errors.Is(err, ErrValidation) {
			writeError(w, http.StatusBadRequest, "invalid pin data")
			return
		}
		writeError(w, http.StatusInternalServerError, "failed to create pin")
		return
	}

	writeJSON(w, http.StatusCreated, pin)
}

func writeJSON(w http.ResponseWriter, status int, payload any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(payload)
}

func writeError(w http.ResponseWriter, status int, message string) {
	writeJSON(w, status, map[string]string{"error": message})
}
