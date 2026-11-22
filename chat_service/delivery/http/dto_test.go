package http

import (
	"encoding/json"
	"testing"
	"time"

	"2026_2_PinPals/chat_service/domain"
)

func TestMessageResponseFromDomainKeepsPublicAuthorFields(t *testing.T) {
	createdAt := time.Date(2026, 11, 23, 12, 30, 0, 0, time.UTC)
	relatedTo := 12
	message := domain.MessageView{
		Message: domain.Message{
			ID:        5,
			ChatID:    9,
			AuthorID:  3,
			Body:      "hello",
			RelatedTo: &relatedTo,
			CreatedAt: createdAt,
		},
		AuthorName: "Ada",
		AuthorTag:  "ada",
	}

	response := MessageResponseFromDomain(message)
	if response.ID != message.ID || response.ChatID != message.ChatID ||
		response.AuthorID != message.AuthorID || response.AuthorName != "Ada" ||
		response.AuthorTag != "ada" || response.Body != message.Body ||
		response.RelatedTo == nil || *response.RelatedTo != relatedTo ||
		!response.CreatedAt.Equal(createdAt) {
		t.Fatalf("MessageResponseFromDomain() = %+v, does not preserve message data", response)
	}

	payload, err := json.Marshal(response)
	if err != nil {
		t.Fatalf("marshal message response: %v", err)
	}
	var decoded map[string]any
	if err := json.Unmarshal(payload, &decoded); err != nil {
		t.Fatalf("unmarshal message response: %v", err)
	}
	if decoded["author_name"] != "Ada" || decoded["author_tag"] != "ada" {
		t.Fatalf("message response omitted author fields: %s", payload)
	}
}

func TestChatResponseIncludesLastMessageView(t *testing.T) {
	chat := domain.Chat{
		ID:   42,
		Name: "Ada и Grace",
		Participant: domain.Participant{
			ID:      2,
			Name:    "Grace",
			UserTag: "grace",
		},
		LastMessage: &domain.MessageView{
			Message:    domain.Message{ID: 10, ChatID: 42, AuthorID: 1, Body: "hello"},
			AuthorName: "Ada",
			AuthorTag:  "ada",
		},
	}

	response := ChatResponseFromDomain(chat)
	if response.LastMessage == nil {
		t.Fatal("ChatResponseFromDomain() omitted last_message")
	}
	if response.LastMessage.AuthorName != "Ada" || response.LastMessage.AuthorTag != "ada" {
		t.Fatalf("last_message author data = %+v", response.LastMessage)
	}
}
