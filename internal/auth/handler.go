package auth

import (
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"strings"

	"2026_2_PinPals/internal/model"
)

type TokenIssuer func(userID int) (string, error)

type Handler struct {
	service     *Service
	tokenIssuer TokenIssuer
}

func NewHandler(service *Service, tokenIssuer TokenIssuer) *Handler {
	return &Handler{service: service, tokenIssuer: tokenIssuer}
}

func (h *Handler) Register(w http.ResponseWriter, r *http.Request) {
	var input model.RegisterInput
	if err := decodeJSON(w, r, &input); err != nil {
		return
	}

	user, err := h.service.Register(r.Context(), input)
	if err != nil {
		switch {
		case errors.Is(err, ErrValidation):
			writeError(w, http.StatusBadRequest, "invalid registration data")
		case errors.Is(err, ErrUserTagExists):
			writeError(w, http.StatusConflict, "user tag already exists")
		default:
			writeError(w, http.StatusInternalServerError, "failed to register user")
		}
		return
	}

	writeJSON(w, http.StatusCreated, map[string]any{"user": user})
}

func (h *Handler) Login(w http.ResponseWriter, r *http.Request) {
	var input model.LoginInput
	if err := decodeJSON(w, r, &input); err != nil {
		return
	}

	response, err := h.service.Login(r.Context(), input, h.tokenIssuer)
	if err != nil {
		switch {
		case errors.Is(err, ErrValidation), errors.Is(err, ErrInvalidCredentials):
			writeError(w, http.StatusUnauthorized, "invalid credentials")
		default:
			writeError(w, http.StatusInternalServerError, "failed to authenticate")
		}
		return
	}

	writeJSON(w, http.StatusOK, response)
}

func decodeJSON(w http.ResponseWriter, r *http.Request, dst any) error {
	contentType := r.Header.Get("Content-Type")
	if contentType != "" && !strings.HasPrefix(contentType, "application/json") {
		writeError(w, http.StatusUnsupportedMediaType, "content type must be application/json")
		return errors.New("invalid content type")
	}

	decoder := json.NewDecoder(r.Body)
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(dst); err != nil {
		writeError(w, http.StatusBadRequest, "invalid json body")
		return err
	}
	var extra any
	if err := decoder.Decode(&extra); err != io.EOF {
		writeError(w, http.StatusBadRequest, "request body must contain one json object")
		return errors.New("extra json value")
	}
	return nil
}

func writeJSON(w http.ResponseWriter, status int, payload any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(payload)
}

func writeError(w http.ResponseWriter, status int, message string) {
	writeJSON(w, status, map[string]string{"error": message})
}
