package http

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"io"
	"log/slog"
	"net/http"
	"net/url"
	"strconv"
	"strings"

	"2026_2_PinPals/chat_service/domain"
	"2026_2_PinPals/pkg/httpx"
	"2026_2_PinPals/pkg/images"
	"2026_2_PinPals/pkg/middleware"
	"github.com/gorilla/websocket"
)

type ChatService interface {
	CreateChat(ctx context.Context, userID, participantID int) (domain.Chat, error)
	ListChats(ctx context.Context, userID int) ([]domain.Chat, error)
	HasAccess(ctx context.Context, userID, chatID int) (bool, error)
	ListMessages(ctx context.Context, userID, chatID, limit, beforeID int) ([]domain.MessageView, error)
	CreateMessage(ctx context.Context, chatID, authorID int, body string, relatedTo *int) (domain.MessageView, error)
}

type HandlerOptions struct {
	AllowedOrigins []string
	BaseURL        string
}

type Handler struct {
	service ChatService
	hub     *Hub
	options HandlerOptions
	origins map[string]struct{}
}

func NewHandler(service ChatService, hub *Hub, options HandlerOptions) *Handler {
	if hub == nil {
		hub = NewHub()
	}
	origins := make(map[string]struct{}, len(options.AllowedOrigins))
	for _, origin := range options.AllowedOrigins {
		origins[strings.TrimRight(origin, "/")] = struct{}{}
	}
	return &Handler{service: service, hub: hub, options: options, origins: origins}
}

// CreateChat godoc
// @Summary Создать личный чат или вернуть существующий
// @Tags Chats
// @Accept json
// @Produce json
// @Security CookieAuth
// @Param request body CreateChatRequest true "Пользователь-собеседник"
// @Success 201 {object} ChatResponse
// @Router /chats [post]
func (h *Handler) CreateChat(w http.ResponseWriter, r *http.Request) {
	userID, ok := middleware.UserIDFromContext(r.Context())
	if !ok {
		httpx.WriteError(w, http.StatusUnauthorized, "необходима авторизация")
		return
	}
	var request CreateChatRequest
	if err := httpx.DecodeJSON(w, r, &request); err != nil {
		return
	}
	chat, err := h.service.CreateChat(r.Context(), userID, request.UserID)
	if err != nil {
		h.writeError(w, r, err)
		return
	}
	h.prepareChat(&chat)
	httpx.WriteJSON(w, http.StatusCreated, ChatResponseFromDomain(chat))
}

// ListChats godoc
// @Summary Получить список чатов текущего пользователя
// @Tags Chats
// @Produce json
// @Security CookieAuth
// @Success 200 {array} ChatResponse
// @Router /chats [get]
func (h *Handler) ListChats(w http.ResponseWriter, r *http.Request) {
	userID, ok := middleware.UserIDFromContext(r.Context())
	if !ok {
		httpx.WriteError(w, http.StatusUnauthorized, "необходима авторизация")
		return
	}
	chats, err := h.service.ListChats(r.Context(), userID)
	if err != nil {
		h.writeError(w, r, err)
		return
	}
	for i := range chats {
		h.prepareChat(&chats[i])
	}
	httpx.WriteJSON(w, http.StatusOK, ChatResponsesFromDomain(chats))
}

// ListMessages godoc
// @Summary Получить историю сообщений чата
// @Tags Chats
// @Produce json
// @Security CookieAuth
// @Param chatID path int true "ID чата"
// @Param limit query int false "Количество сообщений (1-100)"
// @Param before_id query int false "Курсор: сообщения с меньшим ID"
// @Success 200 {array} MessageResponse
// @Router /chats/{chatID}/messages [get]
func (h *Handler) ListMessages(w http.ResponseWriter, r *http.Request) {
	userID, ok := middleware.UserIDFromContext(r.Context())
	if !ok {
		httpx.WriteError(w, http.StatusUnauthorized, "необходима авторизация")
		return
	}
	chatID, err := pathID(r, "chatID")
	if err != nil {
		httpx.WriteError(w, http.StatusBadRequest, "некорректный ID чата")
		return
	}
	limit := 50
	if value := r.URL.Query().Get("limit"); value != "" {
		limit, err = strconv.Atoi(value)
		if err != nil {
			httpx.WriteError(w, http.StatusBadRequest, "некорректный limit")
			return
		}
	}
	beforeID := 0
	if value := r.URL.Query().Get("before_id"); value != "" {
		beforeID, err = strconv.Atoi(value)
		if err != nil {
			httpx.WriteError(w, http.StatusBadRequest, "некорректный before_id")
			return
		}
	}
	messages, err := h.service.ListMessages(r.Context(), userID, chatID, limit, beforeID)
	if err != nil {
		h.writeError(w, r, err)
		return
	}
	httpx.WriteJSON(w, http.StatusOK, MessageResponsesFromDomain(messages))
}

// SendMessage godoc
// @Summary Отправить сообщение в чат
// @Tags Chats
// @Accept json
// @Produce json
// @Security CookieAuth
// @Param chatID path int true "ID чата"
// @Param request body SendMessageRequest true "Текст сообщения"
// @Success 201 {object} MessageResponse
// @Router /chats/{chatID}/messages [post]
func (h *Handler) SendMessage(w http.ResponseWriter, r *http.Request) {
	userID, ok := middleware.UserIDFromContext(r.Context())
	if !ok {
		httpx.WriteError(w, http.StatusUnauthorized, "необходима авторизация")
		return
	}
	chatID, err := pathID(r, "chatID")
	if err != nil {
		httpx.WriteError(w, http.StatusBadRequest, "некорректный ID чата")
		return
	}
	var request SendMessageRequest
	if err := httpx.DecodeJSON(w, r, &request); err != nil {
		return
	}
	request, valid := request.normalized()
	if !valid {
		httpx.WriteError(w, http.StatusBadRequest, "некорректное сообщение")
		return
	}
	message, err := h.service.CreateMessage(r.Context(), chatID, userID, request.Body, request.RelatedTo)
	if err != nil {
		h.writeError(w, r, err)
		return
	}
	h.broadcastMessage(chatID, message)
	httpx.WriteJSON(w, http.StatusCreated, MessageResponseFromDomain(message))
}

// WebSocket godoc
// @Summary Открыть WebSocket-соединение чата
// @Tags Chats
// @Security CookieAuth
// @Param chatID path int true "ID чата"
// @Router /chats/{chatID}/ws [get]
func (h *Handler) WebSocket(w http.ResponseWriter, r *http.Request) {
	userID, ok := middleware.UserIDFromContext(r.Context())
	if !ok {
		httpx.WriteError(w, http.StatusUnauthorized, "необходима авторизация")
		return
	}
	chatID, err := pathID(r, "chatID")
	if err != nil {
		httpx.WriteError(w, http.StatusBadRequest, "некорректный ID чата")
		return
	}
	// Authorization is checked before the HTTP connection is upgraded.
	allowed, err := h.service.HasAccess(r.Context(), userID, chatID)
	if err != nil {
		h.writeError(w, r, err)
		return
	}
	if !allowed {
		httpx.WriteError(w, http.StatusNotFound, "чат не найден")
		return
	}

	upgrader := websocket.Upgrader{
		ReadBufferSize:  1024,
		WriteBufferSize: 1024,
		CheckOrigin:     h.checkOrigin,
	}
	conn, err := upgrader.Upgrade(w, r, nil)
	if err != nil {
		middleware.LoggerFromContext(r.Context()).WarnContext(r.Context(), "websocket upgrade failed", "error", err)
		return
	}

	ctx, cancel := context.WithCancel(r.Context())
	wsClient := h.hub.register(chatID, conn, cancel)
	if wsClient == nil {
		cancel()
		return
	}
	logger := middleware.LoggerFromContext(r.Context())
	go wsClient.writePump(ctx)
	wsClient.readPump(ctx, func(payload []byte) {
		h.handleSocketMessage(ctx, logger, wsClient, userID, chatID, payload)
	})
	h.hub.unregister(wsClient)
	logger.InfoContext(ctx, "websocket connection closed", "chat_id", chatID, "user_id", userID)
}

func (h *Handler) handleSocketMessage(ctx context.Context, logger *slog.Logger, wsClient *client, userID, chatID int, payload []byte) {
	var request SendMessageRequest
	decoder := json.NewDecoder(bytes.NewReader(payload))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&request); err != nil {
		wsClient.enqueue(socketError("некорректный JSON"))
		return
	}
	var extra any
	if err := decoder.Decode(&extra); err != io.EOF {
		wsClient.enqueue(socketError("некорректный JSON"))
		return
	}
	if request.Type != "message" {
		wsClient.enqueue(socketError("неподдерживаемый тип события"))
		return
	}
	request, valid := request.normalized()
	if !valid {
		wsClient.enqueue(socketError("некорректное сообщение"))
		return
	}
	message, err := h.service.CreateMessage(ctx, chatID, userID, request.Body, request.RelatedTo)
	if err != nil {
		logger.WarnContext(ctx, "websocket message rejected", "chat_id", chatID, "user_id", userID, "error", err)
		switch {
		case errors.Is(err, domain.ErrValidation):
			wsClient.enqueue(socketError("некорректное сообщение"))
		case errors.Is(err, domain.ErrMessageNotFound):
			wsClient.enqueue(socketError("сообщение для ответа не найдено"))
		default:
			wsClient.enqueue(socketError("не удалось отправить сообщение"))
		}
		return
	}
	h.broadcastMessage(chatID, message)
}

func (h *Handler) prepareChat(chat *domain.Chat) {
	if chat.Participant.AvatarURL == nil {
		return
	}
	avatarURL := images.PublicURL(h.options.BaseURL, *chat.Participant.AvatarURL)
	chat.Participant.AvatarURL = &avatarURL
}

func (h *Handler) broadcastMessage(chatID int, message domain.MessageView) {
	payload, err := json.Marshal(map[string]any{"type": "message", "message": MessageResponseFromDomain(message)})
	if err != nil {
		return
	}
	h.hub.broadcast(chatID, payload)
}

func socketError(message string) []byte {
	payload, _ := json.Marshal(map[string]string{"type": "error", "error": message})
	return payload
}

func (h *Handler) checkOrigin(r *http.Request) bool {
	origin := strings.TrimRight(strings.TrimSpace(r.Header.Get("Origin")), "/")
	if origin == "" {
		return true // non-browser clients usually omit Origin
	}
	if _, ok := h.origins[origin]; ok {
		return true
	}
	// Same-origin deployments need not repeat the host in CORS_ALLOWED_ORIGINS.
	parsed, err := url.Parse(origin)
	return err == nil && parsed.Host != "" && strings.EqualFold(parsed.Host, r.Host)
}

func (h *Handler) writeError(w http.ResponseWriter, r *http.Request, err error) {
	switch {
	case errors.Is(err, domain.ErrValidation):
		httpx.WriteError(w, http.StatusBadRequest, "некорректные данные")
	case errors.Is(err, domain.ErrUserNotFound):
		httpx.WriteError(w, http.StatusNotFound, "пользователь не найден")
	case errors.Is(err, domain.ErrChatNotFound), errors.Is(err, domain.ErrMessageNotFound):
		httpx.WriteError(w, http.StatusNotFound, "чат или сообщение не найдено")
	default:
		middleware.LoggerFromContext(r.Context()).ErrorContext(r.Context(), "chat request failed", "error", err, "path", r.URL.Path)
		httpx.WriteError(w, http.StatusInternalServerError, "не удалось выполнить запрос к чату")
	}
}

func pathID(r *http.Request, key string) (int, error) {
	value, err := strconv.Atoi(r.PathValue(key))
	if err != nil || value < 1 {
		return 0, errors.New("invalid path id")
	}
	return value, nil
}
