package http

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"mime"
	"mime/multipart"
	"net/http"
	"net/url"
	"path/filepath"
	"strconv"
	"strings"

	"2026_2_PinPals/pkg/httpx"
	"2026_2_PinPals/pkg/images"
	"2026_2_PinPals/pkg/middleware"
	"2026_2_PinPals/user_service/domain"
	"github.com/go-playground/validator/v10"
)

const maxProfileFormSize = images.MaxUploadSize + (1 << 20)

type UserService interface {
	GetProfile(ctx context.Context, userID int) (*domain.User, error)
	UpdateProfile(ctx context.Context, userID int, input domain.UpdateProfileInput) (*domain.User, error)
	DeleteProfile(ctx context.Context, userID int) error
}

// UploadStorage uses io.Reader so both multipart files and test readers can be used.
type UploadStorage interface {
	Save(source io.Reader) (string, error)
	Delete(filename string) error
}

type HandlerOptions struct {
	Validator    *validator.Validate
	Storage      UploadStorage
	ImageBaseURL string
}

type Handler struct {
	service      UserService
	validator    *validator.Validate
	storage      UploadStorage
	imageBaseURL string
}

func NewHandler(service UserService, options ...HandlerOptions) *Handler {
	v := validator.New()
	var storage UploadStorage
	var baseURL string
	if len(options) > 0 {
		if options[0].Validator != nil {
			v = options[0].Validator
		}
		storage = options[0].Storage
		baseURL = strings.TrimRight(options[0].ImageBaseURL, "/")
	}
	return &Handler{service: service, validator: v, storage: storage, imageBaseURL: baseURL}
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
	h.writeUser(w, http.StatusOK, user)
}

// UpdateMe godoc
// @Summary Изменить профиль текущего пользователя
// @Tags Users
// @Accept json
// @Accept mpfd
// @Produce json
// @Param name formData string false "Имя"
// @Param user_tag formData string false "Тег пользователя"
// @Param description formData string false "Описание профиля"
// @Param avatar formData file false "Файл аватара (JPEG, PNG, GIF, WEBP; до 10 MiB)"
// @Param remove_avatar formData bool false "Удалить текущий аватар"
// @Success 200 {object} UserResponse
// @Security CookieAuth
// @Router /users/me [patch]
func (h *Handler) UpdateMe(w http.ResponseWriter, r *http.Request) {
	userID, ok := middleware.UserIDFromContext(r.Context())
	if !ok {
		httpx.WriteError(w, http.StatusUnauthorized, "необходима авторизация")
		return
	}

	var req UpdateProfileRequest
	var avatar multipart.File
	var removeAvatar bool
	contentType, _, _ := mime.ParseMediaType(r.Header.Get("Content-Type"))
	if contentType == "multipart/form-data" {
		r.Body = http.MaxBytesReader(w, r.Body, maxProfileFormSize)
		if err := r.ParseMultipartForm(maxProfileFormSize); err != nil {
			var maxErr *http.MaxBytesError
			if errors.As(err, &maxErr) {
				httpx.WriteError(w, http.StatusRequestEntityTooLarge, "размер запроса слишком велик")
			} else {
				httpx.WriteError(w, http.StatusBadRequest, "некорректная форма профиля")
			}
			return
		}
		if r.MultipartForm == nil {
			httpx.WriteError(w, http.StatusBadRequest, "пустая форма профиля")
			return
		}
		defer r.MultipartForm.RemoveAll()
		allowedFields := map[string]struct{}{
			"name": {}, "user_tag": {}, "description": {},
			"clear_description": {}, "remove_avatar": {},
		}
		for field, values := range r.MultipartForm.Value {
			if _, allowed := allowedFields[field]; !allowed || len(values) != 1 {
				httpx.WriteError(w, http.StatusBadRequest, "некорректные поля формы профиля")
				return
			}
		}
		for field, files := range r.MultipartForm.File {
			if field != "avatar" || len(files) != 1 {
				httpx.WriteError(w, http.StatusBadRequest, "допускается один файл в поле avatar")
				return
			}
		}
		if values, ok := r.MultipartForm.Value["name"]; ok && len(values) > 0 {
			req.Name = &values[0]
		}
		if values, ok := r.MultipartForm.Value["user_tag"]; ok && len(values) > 0 {
			req.UserTag = &values[0]
		}
		if values, ok := r.MultipartForm.Value["description"]; ok && len(values) > 0 {
			encoded, _ := json.Marshal(values[0])
			req.Description = encoded
		}
		if values := r.MultipartForm.Value["clear_description"]; len(values) > 0 {
			value, err := strconv.ParseBool(values[0])
			if err != nil {
				httpx.WriteError(w, http.StatusBadRequest, "некорректное значение clear_description")
				return
			}
			if value {
				req.Description = json.RawMessage("null")
			}
		}
		if values := r.MultipartForm.Value["remove_avatar"]; len(values) > 0 {
			value, err := strconv.ParseBool(values[0])
			if err != nil {
				httpx.WriteError(w, http.StatusBadRequest, "некорректное значение remove_avatar")
				return
			}
			removeAvatar = value
		}
		file, _, err := r.FormFile("avatar")
		if err == nil {
			avatar = file
			defer avatar.Close()
		} else if !errors.Is(err, http.ErrMissingFile) {
			httpx.WriteError(w, http.StatusBadRequest, "не удалось прочитать файл аватара")
			return
		}
	} else if contentType == "application/json" || contentType == "" {
		if err := httpx.DecodeJSON(w, r, &req); err != nil {
			return
		}
	} else {
		httpx.WriteError(w, http.StatusUnsupportedMediaType, "поддерживаются JSON и multipart/form-data")
		return
	}

	if avatar != nil && removeAvatar {
		httpx.WriteError(w, http.StatusBadRequest, "нельзя одновременно загрузить и удалить аватар")
		return
	}
	if !req.HasChanges() && avatar == nil && !removeAvatar {
		httpx.WriteError(w, http.StatusBadRequest, "не указаны поля для изменения")
		return
	}
	if err := req.Validate(h.validator); err != nil {
		httpx.WriteError(w, http.StatusBadRequest, "некорректные данные профиля")
		return
	}
	input, err := req.ToDomain()
	if err != nil {
		httpx.WriteError(w, http.StatusBadRequest, "некорректные данные профиля")
		return
	}

	var newFilename string
	if avatar != nil {
		if h.storage == nil {
			httpx.WriteError(w, http.StatusServiceUnavailable, "загрузка аватара временно недоступна")
			return
		}
		newFilename, err = h.storage.Save(avatar)
		if err != nil {
			switch {
			case errors.Is(err, images.ErrInvalidImage):
				httpx.WriteError(w, http.StatusBadRequest, "поддерживаются только изображения JPEG, PNG, GIF и WEBP")
			case errors.Is(err, images.ErrImageTooLarge):
				httpx.WriteError(w, http.StatusRequestEntityTooLarge, "размер изображения не должен превышать 10 МиБ")
			default:
				middleware.LoggerFromContext(r.Context()).ErrorContext(r.Context(), "failed to store profile avatar", "error", err)
				httpx.WriteError(w, http.StatusInternalServerError, "не удалось сохранить аватар")
			}
			return
		}
		input.AvatarURLSet = true
		input.AvatarURL = &newFilename
	} else if removeAvatar {
		input.AvatarURLSet = true
		input.AvatarURL = nil
	}

	var previousAvatar *string
	if avatar != nil || removeAvatar {
		previous, getErr := h.service.GetProfile(r.Context(), userID)
		if getErr != nil {
			if newFilename != "" {
				_ = h.storage.Delete(newFilename)
			}
			h.writeError(w, r, getErr, "не удалось обновить профиль")
			return
		}
		if previous == nil {
			if newFilename != "" {
				_ = h.storage.Delete(newFilename)
			}
			httpx.WriteError(w, http.StatusInternalServerError, "не удалось обновить профиль")
			return
		}
		previousAvatar = previous.AvatarURL
	}

	user, err := h.service.UpdateProfile(r.Context(), userID, input)
	if err != nil {
		if newFilename != "" {
			if cleanupErr := h.storage.Delete(newFilename); cleanupErr != nil {
				middleware.LoggerFromContext(r.Context()).ErrorContext(r.Context(), "failed to clean up unreferenced avatar", "error", cleanupErr)
			}
		}
		h.writeError(w, r, err, "не удалось обновить профиль")
		return
	}
	if previousAvatar != nil && (newFilename != "" || removeAvatar) && (input.AvatarURL == nil || *previousAvatar != *input.AvatarURL) && h.storage != nil && isStoredImageName(*previousAvatar) {
		if err := h.storage.Delete(*previousAvatar); err != nil {
			middleware.LoggerFromContext(r.Context()).WarnContext(r.Context(), "failed to delete previous avatar", "error", err)
		}
	}
	h.writeUser(w, http.StatusOK, user)
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
	w.WriteHeader(http.StatusNoContent)
}

func (h *Handler) writeUser(w http.ResponseWriter, status int, user *domain.User) {
	response := UserResponseFromDomain(*user)
	if response.AvatarURL != nil {
		value := h.publicImageURL(*response.AvatarURL)
		response.AvatarURL = &value
	}
	httpx.WriteJSON(w, status, response)
}

func (h *Handler) publicImageURL(value string) string {
	parsed, err := url.Parse(value)
	if err == nil && (parsed.Scheme == "http" || parsed.Scheme == "https") && parsed.Host != "" {
		return value
	}
	if h.imageBaseURL == "" {
		return value
	}
	return h.imageBaseURL + "/images/" + url.PathEscape(value)
}

func isStoredImageName(value string) bool {
	return value != "" && filepath.Base(value) == value && !strings.Contains(value, "://")
}

func (h *Handler) writeError(w http.ResponseWriter, r *http.Request, err error, fallback string) {
	switch {
	case errors.Is(err, domain.ErrValidation):
		httpx.WriteError(w, http.StatusBadRequest, "некорректные данные профиля")
	case errors.Is(err, domain.ErrUserNotFound):
		httpx.WriteError(w, http.StatusNotFound, "пользователь не найден")
	case errors.Is(err, domain.ErrUserTagExists):
		httpx.WriteJSON(w, http.StatusConflict, map[string]any{"error": "конфликт", "fields": map[string]string{"user_tag": "уже существует"}})
	default:
		middleware.LoggerFromContext(r.Context()).ErrorContext(r.Context(), "user profile request failed", "error", err, "path", r.URL.Path)
		httpx.WriteError(w, http.StatusInternalServerError, fallback)
	}
}
