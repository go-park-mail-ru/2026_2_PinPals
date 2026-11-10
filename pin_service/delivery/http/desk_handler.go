package http

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"mime"
	"mime/multipart"
	"net/http"
	"strconv"
	"strings"
	"time"

	"2026_2_PinPals/pin_service/domain"
	"2026_2_PinPals/pkg/httpx"
	"2026_2_PinPals/pkg/images"
	"2026_2_PinPals/pkg/middleware"
)

type DeskService interface {
	Create(context.Context, int, domain.CreateDeskInput) (domain.Desk, error)
	ListOwn(context.Context, int) ([]domain.Desk, error)
	ListPublic(context.Context, int, int) ([]domain.Desk, error)
	GetWithPins(context.Context, int, int) (domain.DeskWithPins, error)
	Update(context.Context, int, int, domain.UpdateDeskInput) (domain.Desk, error)
	Delete(context.Context, int, int) error
	AddPin(context.Context, int, int, int) error
	RemovePin(context.Context, int, int, int) error
}

type DeskImageStorage interface {
	Save(io.Reader) (string, error)
	Delete(string) error
}

type DeskHandler struct {
	service      DeskService
	baseURL      string
	imageStorage DeskImageStorage
}

func NewDeskHandler(service DeskService, baseURL string, storage ...DeskImageStorage) *DeskHandler {
	h := &DeskHandler{service: service, baseURL: baseURL}
	if len(storage) > 0 {
		h.imageStorage = storage[0]
	}
	return h
}

type DeskResponse struct {
	ID           int           `json:"desk_id"`
	CreatorID    int           `json:"creator_id"`
	Name         string        `json:"name"`
	Description  *string       `json:"description,omitempty"`
	AvatarImgURL *string       `json:"avatar_img_url,omitempty"`
	EntryStatus  int16         `json:"entry_status"`
	CreatedAt    time.Time     `json:"created_at"`
	Pins         []PinResponse `json:"pins,omitempty"`
}

func (h *DeskHandler) response(d domain.Desk, pins []domain.Pin) DeskResponse {
	avatarURL := d.AvatarImgURL
	if avatarURL != nil {
		publicURL := images.PublicURL(h.baseURL, *avatarURL)
		avatarURL = &publicURL
	}
	out := DeskResponse{
		ID: d.ID, CreatorID: d.CreatorID, Name: d.Name,
		Description: d.Description, AvatarImgURL: avatarURL,
		EntryStatus: d.EntryStatus, CreatedAt: d.CreatedAt,
	}
	if pins != nil {
		out.Pins = make([]PinResponse, len(pins))
		for i, pin := range pins {
			out.Pins[i] = PinResponseFromDomain(pin, h.baseURL)
		}
	}
	return out
}

func requestMediaType(r *http.Request) (string, error) {
	contentType := r.Header.Get("Content-Type")
	if contentType == "" {
		return "application/json", nil
	}
	mediaType, _, err := mime.ParseMediaType(contentType)
	return mediaType, err
}

func (h *DeskHandler) parseMultipart(
	w http.ResponseWriter,
	r *http.Request,
	allowedFields map[string]bool,
) (map[string]string, *multipart.FileHeader, bool) {
	r.Body = http.MaxBytesReader(w, r.Body, images.MaxUploadSize+(1<<20))
	if err := r.ParseMultipartForm(1 << 20); err != nil {
		var maxErr *http.MaxBytesError
		if errors.As(err, &maxErr) {
			httpx.WriteError(w, http.StatusRequestEntityTooLarge, "Размер запроса слишком велик")
		} else {
			httpx.WriteError(w, http.StatusBadRequest, "Некорректная multipart-форма")
		}
		return nil, nil, false
	}
	if r.MultipartForm == nil {
		httpx.WriteError(w, http.StatusBadRequest, "Некорректная multipart-форма")
		return nil, nil, false
	}
	values := make(map[string]string, len(r.MultipartForm.Value))
	for key, entries := range r.MultipartForm.Value {
		if !allowedFields[key] || len(entries) != 1 {
			r.MultipartForm.RemoveAll()
			httpx.WriteError(w, http.StatusBadRequest, "Некорректные поля доски")
			return nil, nil, false
		}
		values[key] = entries[0]
	}
	var avatar *multipart.FileHeader
	for key, entries := range r.MultipartForm.File {
		if key != "avatar" || len(entries) != 1 {
			r.MultipartForm.RemoveAll()
			httpx.WriteError(w, http.StatusBadRequest, "Допускается один файл в поле avatar")
			return nil, nil, false
		}
		avatar = entries[0]
	}
	return values, avatar, true
}

func (h *DeskHandler) saveAvatar(w http.ResponseWriter, r *http.Request, header *multipart.FileHeader) (string, bool) {
	if header == nil {
		return "", true
	}
	if header.Size > images.MaxUploadSize {
		httpx.WriteError(w, http.StatusRequestEntityTooLarge, "Размер изображения не должен превышать 10 МиБ")
		return "", false
	}
	if h.imageStorage == nil {
		httpx.WriteError(w, http.StatusInternalServerError, "Загрузка аватара временно недоступна")
		return "", false
	}
	file, err := header.Open()
	if err != nil {
		httpx.WriteError(w, http.StatusBadRequest, "Не удалось прочитать файл аватара")
		return "", false
	}
	defer file.Close()
	filename, err := h.imageStorage.Save(file)
	if err != nil {
		switch {
		case errors.Is(err, images.ErrImageTooLarge):
			httpx.WriteError(w, http.StatusRequestEntityTooLarge, "Размер изображения не должен превышать 10 МиБ")
		case errors.Is(err, images.ErrInvalidImage):
			httpx.WriteError(w, http.StatusBadRequest, "Поддерживаются только изображения JPEG, PNG, GIF и WEBP")
		default:
			middleware.LoggerFromContext(r.Context()).ErrorContext(r.Context(), "failed to save desk avatar", "error", err)
			httpx.WriteError(w, http.StatusInternalServerError, "Не удалось сохранить аватар")
		}
		return "", false
	}
	return filename, true
}

// Create godoc
// @Summary Создать доску
// @Tags Desks
// @Accept json
// @Accept mpfd
// @Produce json
// @Param name formData string false "Название доски"
// @Param description formData string false "Описание"
// @Param entry_status formData int false "Статус доступа: 0, 1 или 2"
// @Param avatar formData file false "Изображение аватара (JPEG, PNG, GIF или WebP, до 10 МиБ)"
// @Success 201 {object} DeskResponse
// @Security CookieAuth
// @Router /desks [post]
func (h *DeskHandler) Create(w http.ResponseWriter, r *http.Request) {
	userID, ok := middleware.UserIDFromContext(r.Context())
	if !ok {
		httpx.WriteError(w, http.StatusUnauthorized, "Не авторизован")
		return
	}
	var req CreateDeskRequest
	var avatarHeader *multipart.FileHeader
	mediaType, err := requestMediaType(r)
	if err != nil {
		httpx.WriteError(w, http.StatusUnsupportedMediaType, "Неподдерживаемый тип содержимого")
		return
	}
	switch mediaType {
	case "application/json":
		if err := httpx.DecodeJSON(w, r, &req); err != nil {
			return
		}
	case "multipart/form-data":
		values, header, ok := h.parseMultipart(w, r, map[string]bool{"name": true, "description": true, "entry_status": true})
		if !ok {
			return
		}
		defer r.MultipartForm.RemoveAll()
		req.Name = values["name"]
		if value, exists := values["description"]; exists {
			req.Description = &value
		}
		if value, exists := values["entry_status"]; exists {
			status, err := strconv.ParseInt(value, 10, 16)
			if err != nil {
				httpx.WriteError(w, http.StatusBadRequest, "Некорректное значение entry_status")
				return
			}
			req.EntryStatus = int16(status)
		}
		avatarHeader = header
	default:
		httpx.WriteError(w, http.StatusUnsupportedMediaType, "Поддерживаются JSON и multipart/form-data")
		return
	}
	if err := req.Validate(); err != nil {
		httpx.WriteError(w, http.StatusBadRequest, "Некорректные данные доски")
		return
	}
	filename, ok := h.saveAvatar(w, r, avatarHeader)
	if !ok {
		return
	}
	input := req.ToDomain()
	if filename != "" {
		input.AvatarImgURL = &filename
	}
	desk, err := h.service.Create(r.Context(), userID, input)
	if err != nil {
		if filename != "" {
			_ = h.imageStorage.Delete(filename)
		}
		h.writeError(w, r, err)
		return
	}
	httpx.WriteJSON(w, http.StatusCreated, h.response(desk, nil))
}

// ListOwn godoc
// @Summary Получить свои доски
// @Tags Desks
// @Produce json
// @Success 200 {object} map[string]interface{}
// @Security CookieAuth
// @Router /desks/me [get]
func (h *DeskHandler) ListOwn(w http.ResponseWriter, r *http.Request) {
	userID, ok := middleware.UserIDFromContext(r.Context())
	if !ok {
		httpx.WriteError(w, http.StatusUnauthorized, "Не авторизован")
		return
	}
	desks, err := h.service.ListOwn(r.Context(), userID)
	if err != nil {
		h.writeError(w, r, err)
		return
	}
	h.writeDesks(w, desks)
}

// ListPublic godoc
// @Summary Получить публичные доски пользователя
// @Tags Desks
// @Produce json
// @Param userID path int true "Идентификатор пользователя"
// @Success 200 {object} map[string]interface{}
// @Router /users/{userID}/desks [get]
func (h *DeskHandler) ListPublic(w http.ResponseWriter, r *http.Request) {
	ownerID, err := strconv.Atoi(r.PathValue("userID"))
	if err != nil || ownerID < 1 {
		httpx.WriteError(w, http.StatusBadRequest, "Некорректный идентификатор пользователя")
		return
	}
	desks, err := h.service.ListPublic(r.Context(), ownerID, 100)
	if err != nil {
		h.writeError(w, r, err)
		return
	}
	h.writeDesks(w, desks)
}

// GetByID godoc
// @Summary Получить доску с пинами
// @Tags Desks
// @Produce json
// @Param deskID path int true "Идентификатор доски"
// @Success 200 {object} DeskResponse
// @Failure 403 {object} map[string]interface{}
// @Failure 404 {object} map[string]interface{}
// @Router /desks/{deskID} [get]
func (h *DeskHandler) GetByID(w http.ResponseWriter, r *http.Request) {
	deskID, err := strconv.Atoi(r.PathValue("deskID"))
	if err != nil || deskID < 1 {
		httpx.WriteError(w, http.StatusBadRequest, "Некорректный идентификатор доски")
		return
	}
	userID, _ := middleware.UserIDFromContext(r.Context())
	result, err := h.service.GetWithPins(r.Context(), userID, deskID)
	if err != nil {
		h.writeError(w, r, err)
		return
	}
	httpx.WriteJSON(w, http.StatusOK, h.response(result.Desk, result.Pins))
}

// Update godoc
// @Summary Изменить доску
// @Tags Desks
// @Accept json
// @Accept mpfd
// @Produce json
// @Param deskID path int true "Идентификатор доски"
// @Param avatar formData file false "Новое изображение аватара"
// @Param remove_avatar formData bool false "Удалить аватар"
// @Success 200 {object} DeskResponse
// @Security CookieAuth
// @Router /desks/{deskID} [patch]
func (h *DeskHandler) Update(w http.ResponseWriter, r *http.Request) {
	userID, ok := middleware.UserIDFromContext(r.Context())
	if !ok {
		httpx.WriteError(w, http.StatusUnauthorized, "Не авторизован")
		return
	}
	deskID, err := strconv.Atoi(r.PathValue("deskID"))
	if err != nil || deskID < 1 {
		httpx.WriteError(w, http.StatusBadRequest, "Некорректный идентификатор доски")
		return
	}

	var req UpdateDeskRequest
	var avatarHeader *multipart.FileHeader
	mediaType, err := requestMediaType(r)
	if err != nil {
		httpx.WriteError(w, http.StatusUnsupportedMediaType, "Неподдерживаемый тип содержимого")
		return
	}
	switch mediaType {
	case "application/json":
		if err := httpx.DecodeJSON(w, r, &req); err != nil {
			return
		}
	case "multipart/form-data":
		values, header, ok := h.parseMultipart(w, r, map[string]bool{"name": true, "description": true, "entry_status": true, "remove_avatar": true})
		if !ok {
			return
		}
		defer r.MultipartForm.RemoveAll()
		if value, exists := values["name"]; exists {
			req.Name = &value
		}
		if value, exists := values["description"]; exists {
			raw, _ := json.Marshal(value)
			req.Description = raw
		}
		if value, exists := values["entry_status"]; exists {
			status, err := strconv.ParseInt(value, 10, 16)
			if err != nil {
				httpx.WriteError(w, http.StatusBadRequest, "Некорректное значение entry_status")
				return
			}
			parsed := int16(status)
			req.EntryStatus = &parsed
		}
		if value, exists := values["remove_avatar"]; exists {
			remove, err := strconv.ParseBool(value)
			if err != nil {
				httpx.WriteError(w, http.StatusBadRequest, "Некорректное значение remove_avatar")
				return
			}
			req.RemoveAvatar = remove
		}
		avatarHeader = header
	default:
		httpx.WriteError(w, http.StatusUnsupportedMediaType, "Поддерживаются JSON и multipart/form-data")
		return
	}
	if avatarHeader != nil && req.RemoveAvatar {
		httpx.WriteError(w, http.StatusBadRequest, "Нельзя одновременно загрузить и удалить аватар")
		return
	}
	if err := req.Validate(); err != nil {
		httpx.WriteError(w, http.StatusBadRequest, "Некорректные данные доски")
		return
	}
	input, err := req.ToDomain()
	if err != nil {
		httpx.WriteError(w, http.StatusBadRequest, "Некорректные данные доски")
		return
	}

	previousAvatar := ""
	if avatarHeader != nil || req.RemoveAvatar {
		old, err := h.service.GetWithPins(r.Context(), userID, deskID)
		if err != nil {
			h.writeError(w, r, err)
			return
		}
		if old.Desk.AvatarImgURL != nil {
			previousAvatar = *old.Desk.AvatarImgURL
		}
	}
	filename, ok := h.saveAvatar(w, r, avatarHeader)
	if !ok {
		return
	}
	if avatarHeader != nil {
		input.AvatarImgURL = &filename
		input.AvatarImgURLSet = true
	}
	desk, err := h.service.Update(r.Context(), userID, deskID, input)
	if err != nil {
		if filename != "" {
			_ = h.imageStorage.Delete(filename)
		}
		h.writeError(w, r, err)
		return
	}
	newAvatar := ""
	if input.AvatarImgURL != nil {
		newAvatar = *input.AvatarImgURL
	}
	if input.AvatarImgURLSet && previousAvatar != "" && previousAvatar != newAvatar &&
		images.IsStoredFilename(previousAvatar) && h.imageStorage != nil {
		if err := h.imageStorage.Delete(previousAvatar); err != nil {
			middleware.LoggerFromContext(r.Context()).ErrorContext(r.Context(), "failed to delete old desk avatar", "filename", previousAvatar, "error", err)
		}
	}
	httpx.WriteJSON(w, http.StatusOK, h.response(desk, nil))
}

// Delete godoc
// @Summary Удалить доску
// @Tags Desks
// @Param deskID path int true "Идентификатор доски"
// @Success 204
// @Security CookieAuth
// @Router /desks/{deskID} [delete]
func (h *DeskHandler) Delete(w http.ResponseWriter, r *http.Request) {
	userID, ok := middleware.UserIDFromContext(r.Context())
	if !ok {
		httpx.WriteError(w, http.StatusUnauthorized, "Не авторизован")
		return
	}
	deskID, err := strconv.Atoi(r.PathValue("deskID"))
	if err != nil || deskID < 1 {
		httpx.WriteError(w, http.StatusBadRequest, "Некорректный идентификатор доски")
		return
	}
	if err := h.service.Delete(r.Context(), userID, deskID); err != nil {
		h.writeError(w, r, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

// AddPin godoc
// @Summary Добавить пин на доску
// @Tags Desks
// @Param deskID path int true "Идентификатор доски"
// @Param pinID path int true "Идентификатор пина"
// @Success 204
// @Security CookieAuth
// @Router /desks/{deskID}/pins/{pinID} [post]
func (h *DeskHandler) AddPin(w http.ResponseWriter, r *http.Request) { h.changePin(w, r, true) }

// RemovePin godoc
// @Summary Удалить пин с доски
// @Tags Desks
// @Param deskID path int true "Идентификатор доски"
// @Param pinID path int true "Идентификатор пина"
// @Success 204
// @Security CookieAuth
// @Router /desks/{deskID}/pins/{pinID} [delete]
func (h *DeskHandler) RemovePin(w http.ResponseWriter, r *http.Request) { h.changePin(w, r, false) }

func (h *DeskHandler) changePin(w http.ResponseWriter, r *http.Request, add bool) {
	userID, ok := middleware.UserIDFromContext(r.Context())
	if !ok {
		httpx.WriteError(w, http.StatusUnauthorized, "Не авторизован")
		return
	}
	deskID, deskErr := strconv.Atoi(r.PathValue("deskID"))
	pinID, pinErr := strconv.Atoi(r.PathValue("pinID"))
	if deskErr != nil || pinErr != nil || deskID < 1 || pinID < 1 {
		httpx.WriteError(w, http.StatusBadRequest, "Некорректные идентификаторы доски и пина")
		return
	}
	var err error
	if add {
		err = h.service.AddPin(r.Context(), userID, deskID, pinID)
	} else {
		err = h.service.RemovePin(r.Context(), userID, deskID, pinID)
	}
	if err != nil {
		h.writeError(w, r, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func (h *DeskHandler) writeDesks(w http.ResponseWriter, desks []domain.Desk) {
	out := make([]DeskResponse, len(desks))
	for i, desk := range desks {
		out[i] = h.response(desk, nil)
	}
	httpx.WriteJSON(w, http.StatusOK, map[string]any{"desks": out})
}

func (h *DeskHandler) writeError(w http.ResponseWriter, r *http.Request, err error) {
	switch {
	case errors.Is(err, domain.ErrValidation):
		httpx.WriteError(w, http.StatusBadRequest, "Некорректные данные доски")
	case errors.Is(err, domain.ErrDeskNotFound):
		httpx.WriteError(w, http.StatusNotFound, "Доска не найдена")
	case errors.Is(err, domain.ErrDeskForbidden):
		httpx.WriteError(w, http.StatusForbidden, "Нет доступа к доске")
	case errors.Is(err, domain.ErrPinNotFound):
		httpx.WriteError(w, http.StatusNotFound, "Пин не найден")
	default:
		middleware.LoggerFromContext(r.Context()).ErrorContext(r.Context(), "Не удалось выполнить запрос к доске", "error", err, "path", strings.TrimSpace(r.URL.Path))
		httpx.WriteError(w, http.StatusInternalServerError, "Не удалось выполнить запрос к доске")
	}
}
