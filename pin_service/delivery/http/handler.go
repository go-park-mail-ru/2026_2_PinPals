package http

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"strconv"
	"strings"

	"github.com/go-playground/validator/v10"

	"2026_2_PinPals/pin_service/domain"
	"2026_2_PinPals/pkg/httpx"
	"2026_2_PinPals/pkg/images"
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
	service      PinService
	imageStorage interface {
		Save(io.Reader) (string, error)
		Delete(string) error
	}
	baseURL          string
	requestValidator *validator.Validate
}

func NewHandler(
	service PinService,
	baseURL string,
	requestValidator *validator.Validate,
	storage ...interface {
		Save(io.Reader) (string, error)
		Delete(string) error
	},
) *Handler {
	if requestValidator == nil {
		requestValidator = validator.New()
	}
	var imageStorage interface {
		Save(io.Reader) (string, error)
		Delete(string) error
	}
	if len(storage) > 0 {
		imageStorage = storage[0]
	}
	return &Handler{
		service:          service,
		imageStorage:     imageStorage,
		baseURL:          baseURL,
		requestValidator: requestValidator,
	}
}

// List godoc
// @Summary Получить список пинов
// @Tags Pins
// @Accept json
// @Produce json
// @Param request body ListRequest true "Параметры пагинации"
// @Success 200 {object} ListResponse
// @Failure 400 {object} map[string]interface{}
// @Router /pins/search [post]
func (h *Handler) List(w http.ResponseWriter, r *http.Request) {
	var req ListRequest
	if err := httpx.DecodeJSON(w, r, &req); err != nil {
		return
	}

	if err := h.requestValidator.Struct(req); err != nil {
		httpx.WriteError(w, http.StatusBadRequest, "Некорректные параметры пагинации")
		return
	}

	limit, cursor := req.ToDomain()

	page, err := h.service.List(r.Context(), limit, cursor)
	if err != nil {
		if errors.Is(err, domain.ErrValidation) {
			httpx.WriteError(w, http.StatusBadRequest, "Некорректные параметры пагинации")
			return
		}

		middleware.LoggerFromContext(r.Context()).ErrorContext(
			r.Context(),
			"failed to get pins",
			"error", err,
		)

		httpx.WriteError(
			w,
			http.StatusInternalServerError,
			"Не удалось получить список пинов",
		)
		return
	}

	httpx.WriteJSON(w, http.StatusOK, ListResponseFromDomain(page, h.baseURL))
}

// GetByID godoc
// @Summary Получить пин по идентификатору
// @Tags Pins
// @Produce json
// @Param pinID path int true "Идентификатор пина"
// @Success 200 {object} PinResponse
// @Failure 404 {object} map[string]interface{}
// @Router /pins/{pinID} [get]
func (h *Handler) GetByID(w http.ResponseWriter, r *http.Request) {
	id, err := strconv.Atoi(r.PathValue("pinID"))
	if err != nil {
		httpx.WriteError(w, http.StatusBadRequest, "Некорректный идентификатор пина")
		return
	}

	pin, err := h.service.GetByID(r.Context(), id)
	if err != nil {
		switch {
		case errors.Is(err, domain.ErrValidation):
			httpx.WriteError(w, http.StatusBadRequest, "Некорректный идентификатор пина")

		case errors.Is(err, domain.ErrPinNotFound):
			httpx.WriteError(w, http.StatusNotFound, "Пин не найден")

		default:
			middleware.LoggerFromContext(r.Context()).ErrorContext(
				r.Context(),
				"failed to get pin",
				"pin_id", id,
				"error", err,
			)

			httpx.WriteError(
				w,
				http.StatusInternalServerError,
				"Не удалось получить пин",
			)
		}
		return
	}

	httpx.WriteJSON(w, http.StatusOK, PinResponseFromDomain(pin, h.baseURL))
}

// Create godoc
// @Summary Создать пин с загружаемым изображением
// @Tags Pins
// @Accept mpfd
// @Produce json
// @Param name formData string true "Название пина"
// @Param description formData string false "Описание"
// @Param tags formData string false "JSON-массив тегов"
// @Param image formData file true "Изображение JPEG, PNG, GIF или WebP (до 10 МиБ)"
// @Success 201 {object} PinResponse
// @Failure 400 {object} map[string]interface{}
// @Failure 401 {object} map[string]interface{}
// @Failure 413 {object} map[string]interface{}
// @Security CookieAuth
// @Router /pins [post]
func (h *Handler) Create(w http.ResponseWriter, r *http.Request) {
	userID, ok := middleware.UserIDFromContext(r.Context())
	if !ok {
		httpx.WriteError(w, http.StatusUnauthorized, "Не авторизован")
		return
	}
	if h.imageStorage == nil {
		httpx.WriteError(w, http.StatusInternalServerError, "Хранилище изображений не настроено")
		return
	}

	// Учитываем multipart overhead и ограничиваем общий размер тела запроса.
	r.Body = http.MaxBytesReader(w, r.Body, images.MaxUploadSize+(1<<20))
	if err := r.ParseMultipartForm(1 << 20); err != nil {
		var maxErr *http.MaxBytesError
		if errors.As(err, &maxErr) {
			httpx.WriteError(w, http.StatusRequestEntityTooLarge, "Тело запроса слишком большое")
		} else {
			httpx.WriteError(w, http.StatusBadRequest, "Некорректная форма загрузки")
		}
		return
	}
	if r.MultipartForm != nil {
		defer r.MultipartForm.RemoveAll()
	}
	file, header, err := r.FormFile("image")
	if err != nil {
		httpx.WriteError(w, http.StatusBadRequest, "Необходимо приложить изображение")
		return
	}
	defer file.Close()
	if header.Size > images.MaxUploadSize {
		httpx.WriteError(w, http.StatusRequestEntityTooLarge, "Изображение превышает 10 МиБ")
		return
	}

	var tags []string
	if rawTags := strings.TrimSpace(r.FormValue("tags")); rawTags != "" {
		if err := json.Unmarshal([]byte(rawTags), &tags); err != nil {
			httpx.WriteError(w, http.StatusBadRequest, "Поле tags должно содержать JSON-массив строк")
			return
		}
	}
	var description *string
	if value, exists := r.MultipartForm.Value["description"]; exists && len(value) > 0 {
		v := value[0]
		description = &v
	}
	req := CreatePinRequest{Name: strings.TrimSpace(r.FormValue("name")), Description: description, Tags: tags}
	if err := h.requestValidator.Struct(req); err != nil {
		httpx.WriteError(w, http.StatusBadRequest, "Некорректные данные пина")
		return
	}

	filename, err := h.imageStorage.Save(file)
	if err != nil {
		switch {
		case errors.Is(err, images.ErrImageTooLarge):
			httpx.WriteError(w, http.StatusRequestEntityTooLarge, "изображение превышает 10 МиБ")
		case errors.Is(err, images.ErrInvalidImage):
			httpx.WriteError(w, http.StatusBadRequest, "недопустимый формат изображения")
		default:
			middleware.LoggerFromContext(r.Context()).ErrorContext(r.Context(), "failed to save image", "error", err)
			httpx.WriteError(w, http.StatusInternalServerError, "failed to save image")
		}
		return
	}
	input := req.ToDomain(filename)
	pin, err := h.service.Create(r.Context(), userID, input)
	if err != nil {
		_ = h.imageStorage.Delete(filename)
		if errors.Is(err, domain.ErrValidation) {
			httpx.WriteError(w, http.StatusBadRequest, "Некорректные данные пина")
			return
		}
		middleware.LoggerFromContext(r.Context()).ErrorContext(r.Context(), "failed to create pin", "user_id", userID, "error", err)
		httpx.WriteError(w, http.StatusInternalServerError, "Не удалось создать пин")
		return
	}
	httpx.WriteJSON(w, http.StatusCreated, PinResponseFromDomain(pin, h.baseURL))
}
