package http

import (
	"context"
	"errors"
	"io"
	"mime"
	"net/http"
	"strconv"
	"strings"
	"unicode/utf8"

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
		httpx.WriteError(w, http.StatusUnauthorized, "необходима авторизация")
		return
	}
	user, err := h.service.GetProfile(r.Context(), userID)
	if err != nil {
		h.writeError(w, r, err, "не удалось получить профиль")
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
		httpx.WriteError(w, http.StatusUnauthorized, "необходима авторизация")
		return
	}

	// Определяем формат запроса.
	contentType := r.Header.Get("Content-Type")
	if contentType == "" {
		contentType = "application/json"
	}

	mediaType, _, err := mime.ParseMediaType(contentType)
	if err != nil {
		httpx.WriteError(
			w,
			http.StatusUnsupportedMediaType,
			"неподдерживаемый тип содержимого",
		)
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

		if err := req.Validate(); err != nil {
			httpx.WriteError(w, http.StatusBadRequest, "некорректные данные профиля")
			return
		}
		input, err = req.ToDomain()
		if err != nil {
			httpx.WriteError(w, http.StatusBadRequest, "некорректные данные профиля")
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
		httpx.WriteError(
			w,
			http.StatusUnsupportedMediaType,
			"поддерживаются JSON и multipart/form-data",
		)
		return
	}

	cleanupUploadedAvatar := func() {
		if uploadedFilename == "" || h.options.ImageStorage == nil {
			return
		}

		if deleteErr := h.options.ImageStorage.Delete(uploadedFilename); deleteErr != nil {
			middleware.LoggerFromContext(r.Context()).ErrorContext(
				r.Context(),
				"failed to delete unused uploaded avatar",
				"filename", uploadedFilename,
				"error", deleteErr,
			)
		}

		uploadedFilename = ""
	}

	user, err := h.service.UpdateProfile(
		r.Context(),
		userID,
		input,
	)
	if err != nil {
		cleanupUploadedAvatar()

		h.writeError(
			w,
			r,
			err,
			"не удалось обновить профиль",
		)
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

	httpx.WriteJSON(
		w,
		http.StatusOK,
		UserResponseFromDomain(*user, h.options.BaseURL),
	)
}

func (h *Handler) parseMultipartProfile(
	w http.ResponseWriter,
	r *http.Request,
	userID int,
) (domain.UpdateProfileInput, string, string, bool) {
	var input domain.UpdateProfileInput
	r.Body = http.MaxBytesReader(w, r.Body, images.MaxUploadSize+(1<<20))
	if err := r.ParseMultipartForm(1 << 20); err != nil {
		var maxErr *http.MaxBytesError
		if errors.As(err, &maxErr) {
			httpx.WriteError(w, http.StatusRequestEntityTooLarge, "размер запроса слишком велик")
		} else {
			httpx.WriteError(w, http.StatusBadRequest, "некорректная форма профиля")
		}
		return input, "", "", false
	}
	if r.MultipartForm == nil {
		httpx.WriteError(w, http.StatusBadRequest, "некорректная форма профиля")
		return input, "", "", false
	}
	defer r.MultipartForm.RemoveAll()
	allowed := map[string]bool{"name": true, "user_tag": true, "description": true, "clear_description": true, "remove_avatar": true}
	for key, values := range r.MultipartForm.Value {
		if !allowed[key] || len(values) != 1 {
			httpx.WriteError(w, http.StatusBadRequest, "некорректные поля формы профиля")
			return input, "", "", false
		}
	}
	read := func(key string) (string, bool) {
		values, ok := r.MultipartForm.Value[key]
		if !ok {
			return "", false
		}
		return values[0], true
	}
	if value, ok := read("name"); ok {
		value = strings.TrimSpace(value)
		if value == "" || utf8.RuneCountInString(value) > 255 {
			httpx.WriteError(w, http.StatusBadRequest, "некорректное имя пользователя")
			return input, "", "", false
		}
		input.Name = &value
	}
	if value, ok := read("user_tag"); ok {
		value = strings.TrimSpace(value)
		if value == "" || utf8.RuneCountInString(value) > 64 {
			httpx.WriteError(w, http.StatusBadRequest, "некорректный тег пользователя")
			return input, "", "", false
		}
		input.UserTag = &value
	}
	if value, ok := read("description"); ok {
		input.DescriptionSet = true
		value = strings.TrimSpace(value)
		if value != "" {
			if utf8.RuneCountInString(value) > 5000 {
				httpx.WriteError(w, http.StatusBadRequest, "описание слишком длинное")
				return input, "", "", false
			}
			input.Description = &value
		}
	}
	if value, ok := read("clear_description"); ok {
		clear, err := strconv.ParseBool(value)
		if err != nil || clear && input.DescriptionSet {
			httpx.WriteError(w, http.StatusBadRequest, "некорректное значение clear_description")
			return input, "", "", false
		}
		if clear {
			input.DescriptionSet = true
		}
	}
	removeAvatar := false
	if value, ok := read("remove_avatar"); ok {
		var err error
		removeAvatar, err = strconv.ParseBool(value)
		if err != nil {
			httpx.WriteError(w, http.StatusBadRequest, "некорректное значение remove_avatar")
			return input, "", "", false
		}
	}
	for key, files := range r.MultipartForm.File {
		if key != "avatar" || len(files) != 1 {
			httpx.WriteError(w, http.StatusBadRequest, "допускается один файл в поле avatar")
			return input, "", "", false
		}
	}
	file, header, fileErr := r.FormFile("avatar")
	hasFile := fileErr == nil
	if fileErr != nil && !errors.Is(fileErr, http.ErrMissingFile) {
		httpx.WriteError(w, http.StatusBadRequest, "не удалось прочитать файл аватара")
		return input, "", "", false
	}
	if removeAvatar && hasFile {
		_ = file.Close()
		httpx.WriteError(w, http.StatusBadRequest, "нельзя одновременно загрузить и удалить аватар")
		return input, "", "", false
	}
	if input.Name == nil && input.UserTag == nil && !input.DescriptionSet && !removeAvatar && !hasFile {
		httpx.WriteError(w, http.StatusBadRequest, "не указаны поля для изменения")
		return input, "", "", false
	}
	if removeAvatar || hasFile {
		oldUser, err := h.service.GetProfile(r.Context(), userID)
		if err != nil {
			h.writeError(w, r, err, "не удалось получить текущий профиль")
			if hasFile {
				_ = file.Close()
			}
			return input, "", "", false
		}
		previousAvatar := ""
		if oldUser.AvatarURL != nil {
			previousAvatar = *oldUser.AvatarURL
		}
		if removeAvatar {
			input.AvatarURLSet = true
			return input, "", previousAvatar, true
		}
		defer file.Close()
		if header.Size > images.MaxUploadSize {
			httpx.WriteError(w, http.StatusRequestEntityTooLarge, "размер изображения не должен превышать 10 МиБ")
			return input, "", "", false
		}
		if h.options.ImageStorage == nil {
			httpx.WriteError(w, http.StatusInternalServerError, "загрузка аватара временно недоступна")
			return input, "", "", false
		}
		filename, err := h.options.ImageStorage.Save(file)
		if err != nil {
			switch {
			case errors.Is(err, images.ErrImageTooLarge):
				httpx.WriteError(w, http.StatusRequestEntityTooLarge, "размер изображения не должен превышать 10 МиБ")
			case errors.Is(err, images.ErrInvalidImage):
				httpx.WriteError(w, http.StatusBadRequest, "поддерживаются только изображения JPEG, PNG, GIF и WEBP")
			default:
				middleware.LoggerFromContext(r.Context()).ErrorContext(r.Context(), "не удалось сохранить аватар", "error", err)
				httpx.WriteError(w, http.StatusInternalServerError, "не удалось сохранить аватар")
			}
			return input, "", "", false
		}
		input.AvatarURL = &filename
		input.AvatarURLSet = true
		return input, filename, previousAvatar, true
	}
	return input, "", "", true
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
		httpx.WriteError(w, http.StatusUnauthorized, "необходима авторизация")
		return
	}
	if err := h.service.DeleteProfile(r.Context(), userID); err != nil {
		h.writeError(w, r, err, "не удалось удалить профиль")
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
		httpx.WriteError(w, http.StatusBadRequest, "некорректные данные профиля")
	case errors.Is(err, domain.ErrUserNotFound):
		httpx.WriteError(w, http.StatusNotFound, "пользователь не найден")
	case errors.Is(err, domain.ErrUserTagExists):
		httpx.WriteJSON(w, http.StatusConflict, map[string]any{"error": "conflict", "fields": map[string]string{"user_tag": "already_exists"}})
	default:
		middleware.LoggerFromContext(r.Context()).ErrorContext(r.Context(), fallback, "error", err, "path", r.URL.Path)
		httpx.WriteError(w, http.StatusInternalServerError, fallback)
	}
}
