package http

import (
	"net/url"
	"strconv"
	"strings"
	"time"
	"unicode/utf8"

	"2026_2_PinPals/chat_service/domain"
)

type CreateChatRequest struct {
	UserID int `json:"user_id"`
}

// Validate rejects invalid transport input before invoking the use case.
func (r CreateChatRequest) Validate() error {
	if r.UserID < 1 {
		return domain.ErrValidation
	}
	return nil
}

type SendMessageRequest struct {
	Type      string `json:"type,omitempty"`
	Body      string `json:"body"`
	RelatedTo *int   `json:"related_to,omitempty"`
}

func (r SendMessageRequest) Validate() error {
	if r.Type != "" && r.Type != "message" {
		return domain.ErrValidation
	}
	body := strings.TrimSpace(r.Body)
	if body == "" || !utf8.ValidString(body) || utf8.RuneCountInString(body) > domain.MaxMessageLength {
		return domain.ErrValidation
	}
	if r.RelatedTo != nil && *r.RelatedTo < 1 {
		return domain.ErrValidation
	}
	return nil
}

func (r SendMessageRequest) normalized() (SendMessageRequest, bool) {
	r.Body = strings.TrimSpace(r.Body)
	return r, r.Validate() == nil
}

// ListMessagesQuery is the validated transport representation of pagination
// parameters accepted by GET /chats/{chatID}/messages.
type ListMessagesQuery struct {
	Limit    int
	BeforeID int
}

func ParseListMessagesQuery(values url.Values) (ListMessagesQuery, error) {
	query := ListMessagesQuery{Limit: 50}
	if rawLimits, exists := values["limit"]; exists {
		if len(rawLimits) != 1 || strings.TrimSpace(rawLimits[0]) == "" {
			return ListMessagesQuery{}, domain.ErrValidation
		}
		limit, err := strconv.Atoi(rawLimits[0])
		if err != nil {
			return ListMessagesQuery{}, domain.ErrValidation
		}
		query.Limit = limit
	}
	if rawBeforeIDs, exists := values["before_id"]; exists {
		if len(rawBeforeIDs) != 1 || strings.TrimSpace(rawBeforeIDs[0]) == "" {
			return ListMessagesQuery{}, domain.ErrValidation
		}
		beforeID, err := strconv.Atoi(rawBeforeIDs[0])
		if err != nil {
			return ListMessagesQuery{}, domain.ErrValidation
		}
		query.BeforeID = beforeID
	}
	if query.Limit < 1 || query.Limit > domain.MaxMessagesPerPage || query.BeforeID < 0 {
		return ListMessagesQuery{}, domain.ErrValidation
	}
	return query, nil
}

// MessageResponse is the stable HTTP/WebSocket representation of a message.
// Author profile data is resolved at read time and is not persisted per message.
type MessageResponse struct {
	ID         int       `json:"message_id"`
	ChatID     int       `json:"chat_id"`
	AuthorID   int       `json:"author_id"`
	AuthorName string    `json:"author_name"`
	AuthorTag  string    `json:"author_tag"`
	Body       string    `json:"body"`
	RelatedTo  *int      `json:"related_to,omitempty"`
	CreatedAt  time.Time `json:"created_at"`
}

func MessageResponseFromDomain(message domain.MessageView) MessageResponse {
	return MessageResponse{
		ID:         message.ID,
		ChatID:     message.ChatID,
		AuthorID:   message.AuthorID,
		AuthorName: message.AuthorName,
		AuthorTag:  message.AuthorTag,
		Body:       message.Body,
		RelatedTo:  message.RelatedTo,
		CreatedAt:  message.CreatedAt,
	}
}

func MessageResponsesFromDomain(messages []domain.MessageView) []MessageResponse {
	responses := make([]MessageResponse, 0, len(messages))
	for _, message := range messages {
		responses = append(responses, MessageResponseFromDomain(message))
	}
	return responses
}

// ChatResponse contains the public chat representation and uses MessageResponse
// for its last_message, keeping HTTP serialization separate from domain models.
type ChatResponse struct {
	ID          int                `json:"chat_id"`
	Name        string             `json:"name"`
	Participant domain.Participant `json:"participant"`
	LastMessage *MessageResponse   `json:"last_message,omitempty"`
	CreatedAt   time.Time          `json:"created_at"`
	UpdatedAt   time.Time          `json:"updated_at"`
}

func ChatResponseFromDomain(chat domain.Chat) ChatResponse {
	response := ChatResponse{
		ID:          chat.ID,
		Name:        chat.Name,
		Participant: chat.Participant,
		CreatedAt:   chat.CreatedAt,
		UpdatedAt:   chat.UpdatedAt,
	}
	if chat.LastMessage != nil {
		message := MessageResponseFromDomain(*chat.LastMessage)
		response.LastMessage = &message
	}
	return response
}

func ChatResponsesFromDomain(chats []domain.Chat) []ChatResponse {
	responses := make([]ChatResponse, 0, len(chats))
	for _, chat := range chats {
		responses = append(responses, ChatResponseFromDomain(chat))
	}
	return responses
}
