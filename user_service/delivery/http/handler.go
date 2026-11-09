package http

import (
	"context"
	"errors"
	"io"
	"mime"
	"net/http"

	"2026_2_PinPals/pkg/httpx"
	"2026_2_PinPals/pkg/images"
	"2026_2_PinPals/pkg/middleware"
	"2026_2_PinPals/user_service/domain"
)

type UserService interface {
	GetProfile(ctx context.Context, userID int) (*domain.User, error)
	UpdateProfile(ctx context.Context, userID int, input domain.UpdateProfileInput) (*domain.User, error)
	DeleteProfile(ctx context.Context, userID int) error
}

// TokenRevoker отзывает JWT на сервере (реализуется middleware.TokenManager).
type TokenRevoker interface {
	Revoke(ctx context.Context, token string) error
}

type HandlerOptions struct {
	TokenRevoker  TokenRevoker
	SecureCookies bool

	BaseURL      string
	ImageStorage ImageStorage
}

type Handler struct {
	service UserService
	options HandlerOptions
}

type ImageStorage interface {
	Save(io.Reader) (string, error)
	Delete(string) error
}

func NewHandler(service UserService, options ...HandlerOptions) *Handler {
	h := &Handler{service: service}
	if len(options) > 0 {
		h.options = options[0]
	}
	return h
}

// GetMe godoc
// @Summary Получить профиль текущего пользователя
// @Tags Users
// @Produce json
// @Success 200 {object} UserResponse
// @Security CookieAuth
// @Router /users/me [get]
func (h *Handler) GetMe(w http.ResponseWriter, r *http.Request) {
	userID, ok := middleware.UserIDFromContext(r.Context())
	if !ok {
		httpx.WriteError(w, http.StatusUnauthorized, "unauthorized")
		return
	}
	user, err := h.service.GetProfile(r.Context(), userID)
	if err != nil {
		h.writeError(w, r, err, "failed to get profile")
		return
	}
	httpx.WriteJSON(w, http.StatusOK, UserResponseFromDomain(*user, h.options.BaseURL))
}

// UpdateMe godoc
// @Summary Изменить профиль текущего пользователя
// @Tags Users
// @Accept json
// @Accept mpfd
// @Produce json
// @Security CookieAuth
// @Router /users/me [patch]
func (h *Handler) UpdateMe(w http.ResponseWriter, r *http.Request) {
	userID, ok := middleware.UserIDFromContext(r.Context())
	if !ok {
		httpx.WriteError(w, http.StatusUnauthorized, "unauthorized")
		return
	}
	contentType := r.Header.Get("Content-Type")
	if contentType == "" {
		contentType = "application/json"
	}
	mediaType, _, err := mime.ParseMediaType(contentType)
	if err != nil {
		httpx.WriteError(w, http.StatusUnsupportedMediaType, "unsupported content type")
		return
	}
	var input domain.UpdateProfileInput
	var uploadedFilename string
	var previousAvatar string
	switch mediaType {
	case "application/json":
		var req UpdateProfileRequest
		if err := httpx.DecodeJSON(w, r, &req); err != nil {
			return
		}
		input, err = req.ToDomain()
		if err != nil {
			httpx.WriteError(w, http.StatusBadRequest, "invalid profile data")
			return
		}
	case "multipart/form-data":
		var parsed bool

		input, uploadedFilename, previousAvatar, parsed =
			h.parseMultipartProfile(w, r, userID)
		if !parsed {
			return
		}
	default:
		httpx.WriteError(w, http.StatusUnsupportedMediaType, "content type must be application/json or multipart/form-data")
		return
	}
	if input.AvatarURLSet && previousAvatar == "" {
		oldUser, err := h.service.GetProfile(r.Context(), userID)
		if err != nil {
			h.writeError(w, r, err, "failed to get old profile")
			return
		}
		if oldUser.AvatarURL != nil {
			previousAvatar = *oldUser.AvatarURL
		}
	}
	user, err := h.service.UpdateProfile(r.Context(), userID, input)
	if err != nil {
		if uploadedFilename != "" && h.options.ImageStorage != nil {
			if deleteErr := h.options.ImageStorage.Delete(uploadedFilename); deleteErr != nil {
				middleware.LoggerFromContext(r.Context()).ErrorContext(
					r.Context(),
					"failed to delete unused uploaded avatar",
					"filename", uploadedFilename,
					"error", deleteErr,
				)
			}
		}
		h.writeError(w, r, err, "failed to update profile")
		return
	}
	newAvatar := ""
	if input.AvatarURL != nil {
		newAvatar = *input.AvatarURL
	}
	if input.AvatarURLSet &&
		previousAvatar != "" &&
		previousAvatar != newAvatar &&
		images.IsStoredFilename(previousAvatar) &&
		h.options.ImageStorage != nil {
		if err := h.options.ImageStorage.Delete(previousAvatar); err != nil {
			middleware.LoggerFromContext(r.Context()).ErrorContext(
				r.Context(),
				"failed to delete old avatar",
				"filename", previousAvatar,
				"error", err,
			)
		}
	}
	httpx.WriteJSON(w, http.StatusOK, UserResponseFromDomain(*user, h.options.BaseURL))
}

func (h *Handler) parseMultipartProfile(
	w http.ResponseWriter,
	r *http.Request,
	userID int,
) (domain.UpdateProfileInput, string, string, bool) {
	var input domain.UpdateProfileInput

	r.Body = http.MaxBytesReader(w,r.Body,images.MaxUploadSize+(1<<20),)

	if err := r.ParseMultipartForm(1 << 20); err != nil {
		var maxErr *http.MaxBytesError

		if errors.As(err, &maxErr) {
			httpx.WriteError(w,http.StatusRequestEntityTooLarge,"request body too large",)
		} else {
			httpx.WriteError(w,http.StatusBadRequest,"invalid multipart form",)
		}

		return input, "", "", false
	}

	if r.MultipartForm != nil {
		defer r.MultipartForm.RemoveAll()
	}

	if values, exists := r.MultipartForm.Value["name"]; exists && len(values) > 0 {
		value := values[0]
		input.Name = &value
	}

	if values, exists := r.MultipartForm.Value["user_tag"]; exists && len(values) > 0 {
		value := values[0]
		input.UserTag = &value
	}

	if values, exists := r.MultipartForm.Value["description"]; exists && len(values) > 0 {
		input.DescriptionSet = true

		if values[0] != "" {
			value := values[0]
			input.Description = &value
		}
	}

	file, header, err := r.FormFile("avatar")

	if errors.Is(err, http.ErrMissingFile) {
		return input, "", "", true
	}

	if err != nil {
		httpx.WriteError(w,http.StatusBadRequest,"invalid avatar file",)
		return input, "", "", false
	}

	defer file.Close()

	if header.Size > images.MaxUploadSize {
		httpx.WriteError(w,http.StatusRequestEntityTooLarge,"avatar exceeds 10 MiB",)
		return input, "", "", false
	}

	if h.options.ImageStorage == nil {
		httpx.WriteError(w,http.StatusInternalServerError,"image storage is not configured",)
		return input, "", "", false
	}

	oldUser, err := h.service.GetProfile(r.Context(), userID)
	if err != nil {
		h.writeError(w,r,err,"failed to get old profile",)
		return input, "", "", false
	}

	previousAvatar := ""

	if oldUser.AvatarURL != nil {
		previousAvatar = *oldUser.AvatarURL
	}

	filename, err := h.options.ImageStorage.Save(file)

	if err != nil {
		switch {
		case errors.Is(err, images.ErrImageTooLarge):
			httpx.WriteError(w,http.StatusRequestEntityTooLarge,"avatar exceeds 10 MiB",)

		case errors.Is(err, images.ErrInvalidImage):
			httpx.WriteError(w,http.StatusBadRequest,"unsupported image format",)

		default:
			middleware.LoggerFromContext(r.Context()).ErrorContext(
				r.Context(),
				"failed to save avatar",
				"error", err,
			)

			httpx.WriteError(w,http.StatusInternalServerError,"failed to save avatar",)
		}

		return input, "", "", false
	}

	input.AvatarURL = &filename
	input.AvatarURLSet = true
	return input, filename, previousAvatar, true
}

// DeleteMe godoc
// @Summary Удалить профиль текущего пользователя
// @Tags Users
// @Success 204
// @Security CookieAuth
// @Router /users/me [delete]
func (h *Handler) DeleteMe(w http.ResponseWriter, r *http.Request) {
	userID, ok := middleware.UserIDFromContext(r.Context())
	if !ok {
		httpx.WriteError(w, http.StatusUnauthorized, "unauthorized")
		return
	}
	if err := h.service.DeleteProfile(r.Context(), userID); err != nil {
		h.writeError(w, r, err, "failed to delete profile")
		return
	}
	// Профиль удалён: отзываем текущий токен и очищаем cookie.
	// Токены остальных сессий отклоняются middleware по признаку удалённого пользователя.
	if h.options.TokenRevoker != nil {
		if token := middleware.TokenFromRequest(r); token != "" {
			if err := h.options.TokenRevoker.Revoke(r.Context(), token); err != nil {
				middleware.LoggerFromContext(r.Context()).ErrorContext(r.Context(), "failed to revoke token after profile deletion", "error", err)
			}
		}
	}
	middleware.ClearAuthCookies(w, h.options.SecureCookies)
	w.WriteHeader(http.StatusNoContent)
}

func (h *Handler) writeError(w http.ResponseWriter, r *http.Request, err error, fallback string) {
	switch {
	case errors.Is(err, domain.ErrValidation):
		httpx.WriteError(w, http.StatusBadRequest, "invalid profile data")
	case errors.Is(err, domain.ErrUserNotFound):
		httpx.WriteError(w, http.StatusNotFound, "user not found")
	case errors.Is(err, domain.ErrUserTagExists):
		httpx.WriteJSON(w, http.StatusConflict, map[string]any{"error": "conflict", "fields": map[string]string{"user_tag": "already_exists"}})
	default:
		middleware.LoggerFromContext(r.Context()).ErrorContext(r.Context(), fallback, "error", err, "path", r.URL.Path)
		httpx.WriteError(w, http.StatusInternalServerError, fallback)
	}
}
