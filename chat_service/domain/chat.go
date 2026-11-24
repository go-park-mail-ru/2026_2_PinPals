package domain

import (
	"errors"
	"time"
)

const (
	MaxMessageLength   = 4000
	MaxMessagesPerPage = 100
)

var (
	ErrValidation      = errors.New("validation error")
	ErrChatNotFound    = errors.New("chat not found")
	ErrUserNotFound    = errors.New("user not found")
	ErrMessageNotFound = errors.New("message not found")
)

type Participant struct {
	ID        int     `json:"user_id"`
	Name      string  `json:"name"`
	UserTag   string  `json:"user_tag"`
	AvatarURL *string `json:"avatar_url,omitempty"`
}

// Message contains only the data persisted in chat_message.
// Author profile fields are deliberately kept out of this entity to avoid
// duplicating user data in every stored message.
type Message struct {
	ID        int       `json:"message_id"`
	ChatID    int       `json:"chat_id"`
	AuthorID  int       `json:"author_id"`
	Body      string    `json:"body"`
	RelatedTo *int      `json:"related_to,omitempty"`
	CreatedAt time.Time `json:"created_at"`
}

// MessageView is a read model enriched with the author's current public
// profile data. The repository fills these fields with a JOIN to the user
// table; they are never stored in chat_message.
type MessageView struct {
	Message
	AuthorName string
	AuthorTag  string
}

type Chat struct {
	ID          int          `json:"chat_id"`
	Name        string       `json:"name"`
	Participant Participant  `json:"participant"`
	LastMessage *MessageView `json:"last_message,omitempty"`
	CreatedAt   time.Time    `json:"created_at"`
	UpdatedAt   time.Time    `json:"updated_at"`
}
