package repository

import (
	"context"
	"testing"
	"time"

	"2026_2_PinPals/chat_service/domain"
	"github.com/pashagolub/pgxmock/v5"
)

func newMockChatRepository(t *testing.T) (*Repository, pgxmock.PgxPoolIface) {
	t.Helper()
	mock, err := pgxmock.NewPool(pgxmock.QueryMatcherOption(pgxmock.QueryMatcherEqual))
	if err != nil {
		t.Fatalf("create pgx mock: %v", err)
	}
	t.Cleanup(mock.Close)
	return &Repository{pool: mock}, mock
}

func assertChatMockExpectations(t *testing.T, mock pgxmock.PgxPoolIface) {
	t.Helper()
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatalf("unmet database expectations: %v", err)
	}
}

func TestCreateChatUsesCanonicalParticipantOrder(t *testing.T) {
	createdAt := time.Date(2026, 11, 23, 10, 0, 0, 0, time.UTC)
	cases := []struct {
		name        string
		userID      int
		peerID      int
		args        []any
		returnedID  int
		participant int
	}{
		{
			name:   "requester has larger id",
			userID: 9, peerID: 2, args: []any{2, 9, 9}, returnedID: 2, participant: 2,
		},
		{
			name:   "requester has smaller id",
			userID: 2, peerID: 9, args: []any{2, 9, 2}, returnedID: 9, participant: 9,
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			repo, mock := newMockChatRepository(t)
			mock.ExpectQuery(queryCreateChat).WithArgs(tc.args...).WillReturnRows(
				pgxmock.NewRows([]string{
					"chat_id", "name", "created_at", "updated_at",
					"user_id", "participant_name", "user_tag", "avatar_url",
				}).AddRow(17, "Ada и Grace", createdAt, createdAt, tc.returnedID, "Peer", "peer", nil),
			)

			chat, err := repo.CreateChat(context.Background(), tc.userID, tc.peerID)
			if err != nil {
				t.Fatalf("CreateChat() error = %v", err)
			}
			if chat.ID != 17 || chat.Participant.ID != tc.participant {
				t.Fatalf("CreateChat() = %+v, want participant %d", chat, tc.participant)
			}
			assertChatMockExpectations(t, mock)
		})
	}
}

func TestListMessagesReturnsChronologicalOrder(t *testing.T) {
	repo, mock := newMockChatRepository(t)
	createdAt := time.Date(2026, 11, 23, 10, 0, 0, 0, time.UTC)
	mock.ExpectQuery(queryHasAccess).
		WithArgs(17, 9).
		WillReturnRows(pgxmock.NewRows([]string{"exists"}).AddRow(true))
	mock.ExpectQuery(queryListMessages).
		WithArgs(17, 0, 2).
		WillReturnRows(pgxmock.NewRows([]string{
			"message_id", "chat_id", "author_id", "author_name", "author_tag", "body", "related_to", "created_at",
		}).
			AddRow(2, 17, 2, "Grace", "grace", "second", nil, createdAt.Add(time.Minute)).
			AddRow(1, 17, 9, "Ada", "ada", "first", nil, createdAt))

	messages, err := repo.ListMessages(context.Background(), 9, 17, 2, 0)
	if err != nil {
		t.Fatalf("ListMessages() error = %v", err)
	}
	if len(messages) != 2 || messages[0].ID != 1 || messages[1].ID != 2 {
		t.Fatalf("ListMessages() returned unexpected order: %+v", messages)
	}
	if messages[0].AuthorName != "Ada" || messages[0].AuthorTag != "ada" ||
		messages[1].AuthorName != "Grace" || messages[1].AuthorTag != "grace" {
		t.Fatalf("ListMessages() did not enrich authors using the user JOIN: %+v", messages)
	}
	assertChatMockExpectations(t, mock)
}

func TestCreateMessageMapsMissingChat(t *testing.T) {
	repo, mock := newMockChatRepository(t)
	mock.ExpectQuery(queryCreateMessage).
		WithArgs(17, 9, "hello", pgxmock.AnyArg()).
		WillReturnRows(pgxmock.NewRows([]string{
			"message_id", "chat_id", "author_id", "author_name", "author_tag", "body", "related_to", "created_at",
		}))
	mock.ExpectQuery(queryHasAccess).
		WithArgs(17, 9).
		WillReturnRows(pgxmock.NewRows([]string{"exists"}).AddRow(false))

	if _, err := repo.CreateMessage(context.Background(), 17, 9, "hello", nil); err != domain.ErrChatNotFound {
		t.Fatalf("CreateMessage() error = %v, want %v", err, domain.ErrChatNotFound)
	}
	assertChatMockExpectations(t, mock)
}
