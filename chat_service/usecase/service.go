package usecase

import (
	"context"
	"strings"
	"unicode/utf8"

	"2026_2_PinPals/chat_service/domain"
)

// ChatRepository describes the persistence operations required by chat use cases.
type ChatRepository interface {
	CreateChat(ctx context.Context, userAID, userBID int) (domain.Chat, error)
	ListChats(ctx context.Context, userID int) ([]domain.Chat, error)
	HasAccess(ctx context.Context, userID, chatID int) (bool, error)
	ListMessages(ctx context.Context, userID, chatID, limit, beforeID int) ([]domain.MessageView, error)
	CreateMessage(ctx context.Context, chatID, authorID int, body string, relatedTo *int) (domain.MessageView, error)
}

type Service struct {
	repository ChatRepository
}

func NewService(repository ChatRepository) *Service {
	return &Service{repository: repository}
}

func (s *Service) CreateChat(ctx context.Context, userID, participantID int) (domain.Chat, error) {
	if userID < 1 || participantID < 1 || userID == participantID {
		return domain.Chat{}, domain.ErrValidation
	}
	return s.repository.CreateChat(ctx, userID, participantID)
}

func (s *Service) ListChats(ctx context.Context, userID int) ([]domain.Chat, error) {
	if userID < 1 {
		return nil, domain.ErrValidation
	}
	return s.repository.ListChats(ctx, userID)
}

func (s *Service) HasAccess(ctx context.Context, userID, chatID int) (bool, error) {
	if userID < 1 || chatID < 1 {
		return false, domain.ErrValidation
	}
	return s.repository.HasAccess(ctx, userID, chatID)
}

func (s *Service) ListMessages(ctx context.Context, userID, chatID, limit, beforeID int) ([]domain.MessageView, error) {
	if userID < 1 || chatID < 1 || limit < 1 || limit > domain.MaxMessagesPerPage || beforeID < 0 {
		return nil, domain.ErrValidation
	}
	return s.repository.ListMessages(ctx, userID, chatID, limit, beforeID)
}

func (s *Service) CreateMessage(ctx context.Context, chatID, authorID int, body string, relatedTo *int) (domain.MessageView, error) {
	body = strings.TrimSpace(body)
	if chatID < 1 || authorID < 1 || body == "" || !utf8.ValidString(body) || utf8.RuneCountInString(body) > domain.MaxMessageLength {
		return domain.MessageView{}, domain.ErrValidation
	}
	if relatedTo != nil && *relatedTo < 1 {
		return domain.MessageView{}, domain.ErrValidation
	}
	return s.repository.CreateMessage(ctx, chatID, authorID, body, relatedTo)
}
