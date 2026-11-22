package usecase

import (
	"context"
	"testing"

	"2026_2_PinPals/chat_service/domain"
)

type repositoryStub struct {
	createMessageBody string
	createMessageErr  error
}

func (r *repositoryStub) CreateChat(context.Context, int, int) (domain.Chat, error) {
	return domain.Chat{}, nil
}
func (r *repositoryStub) ListChats(context.Context, int) ([]domain.Chat, error) {
	return []domain.Chat{}, nil
}
func (r *repositoryStub) HasAccess(context.Context, int, int) (bool, error) { return true, nil }
func (r *repositoryStub) ListMessages(context.Context, int, int, int, int) ([]domain.MessageView, error) {
	return []domain.MessageView{}, nil
}
func (r *repositoryStub) CreateMessage(_ context.Context, _, _ int, body string, _ *int) (domain.MessageView, error) {
	r.createMessageBody = body
	return domain.MessageView{Message: domain.Message{Body: body}}, r.createMessageErr
}

func TestCreateMessageTrimsBody(t *testing.T) {
	repo := &repositoryStub{}
	service := NewService(repo)

	message, err := service.CreateMessage(context.Background(), 4, 9, "  hello  ", nil)
	if err != nil {
		t.Fatalf("CreateMessage() error = %v", err)
	}
	if repo.createMessageBody != "hello" || message.Body != "hello" {
		t.Fatalf("message body was not normalized: repo=%q response=%q", repo.createMessageBody, message.Body)
	}
}

func TestCreateMessageRejectsInvalidInput(t *testing.T) {
	repo := &repositoryStub{}
	service := NewService(repo)

	cases := []struct {
		name     string
		chatID   int
		authorID int
		body     string
	}{
		{name: "empty body", chatID: 1, authorID: 2, body: "  "},
		{name: "invalid chat", chatID: 0, authorID: 2, body: "hello"},
		{name: "invalid author", chatID: 1, authorID: 0, body: "hello"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if _, err := service.CreateMessage(context.Background(), tc.chatID, tc.authorID, tc.body, nil); err != domain.ErrValidation {
				t.Fatalf("CreateMessage() error = %v, want %v", err, domain.ErrValidation)
			}
		})
	}
}

func TestListMessagesRejectsInvalidPagination(t *testing.T) {
	service := NewService(&repositoryStub{})
	if _, err := service.ListMessages(context.Background(), 1, 1, 101, 0); err != domain.ErrValidation {
		t.Fatalf("ListMessages() error = %v, want %v", err, domain.ErrValidation)
	}
}
