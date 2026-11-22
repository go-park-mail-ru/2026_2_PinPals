package repository

import (
	"context"
	"errors"

	"2026_2_PinPals/chat_service/domain"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/jackc/pgx/v5/pgxpool"
)

// poolDB is the subset of pgxpool.Pool used by Repository. It also allows
// repository tests to exercise SQL with pgxmock without a live database.
type poolDB interface {
	Query(context.Context, string, ...any) (pgx.Rows, error)
	QueryRow(context.Context, string, ...any) pgx.Row
}

type Repository struct {
	pool poolDB
}

func NewRepository(pool *pgxpool.Pool) *Repository {
	return &Repository{pool: pool}
}

const queryCreateChat = `
	WITH upserted AS (
		INSERT INTO chat (user_a_id, user_b_id, name)
		SELECT $1, $2, ua.name || ' и ' || ub.name
		FROM "user" AS ua
		JOIN "user" AS ub ON ub.user_id = $2
		WHERE ua.user_id = $1
		  AND ua.deleted_at IS NULL
		  AND ub.deleted_at IS NULL
		ON CONFLICT (user_a_id, user_b_id) DO UPDATE
		SET name = EXCLUDED.name,
		    deleted = false,
		    deleted_at = NULL,
		    updated_at = now()
		RETURNING chat_id, name, created_at, updated_at, user_a_id, user_b_id
	)
	SELECT
		c.chat_id, c.name, c.created_at, c.updated_at,
		peer.user_id, peer.name, peer.user_tag, peer.avatar_url
	FROM upserted AS c
	JOIN "user" AS peer
	  ON peer.user_id = CASE WHEN c.user_a_id = $3 THEN c.user_b_id ELSE c.user_a_id END
`

const queryListChats = `
	SELECT
		c.chat_id, c.name, c.created_at, c.updated_at,
		peer.user_id, peer.name, peer.user_tag, peer.avatar_url,
		(lm.message_id IS NOT NULL),
		COALESCE(lm.message_id, 0), COALESCE(lm.author_id, 0),
		COALESCE(lm.author_name, ''), COALESCE(lm.author_tag, ''),
		COALESCE(lm.body, ''), COALESCE(lm.created_at, c.updated_at)
	FROM chat AS c
	JOIN "user" AS peer
	  ON peer.user_id = CASE WHEN c.user_a_id = $1 THEN c.user_b_id ELSE c.user_a_id END
	 AND peer.deleted_at IS NULL
	LEFT JOIN LATERAL (
		SELECT m.message_id, m.author_id, u.name AS author_name,
		       u.user_tag AS author_tag, m.body, m.created_at
		FROM chat_message AS m
		JOIN "user" AS u ON u.user_id = m.author_id
		WHERE m.chat_id = c.chat_id AND m.deleted = false
		ORDER BY m.message_id DESC
		LIMIT 1
	) AS lm ON true
	WHERE (c.user_a_id = $1 OR c.user_b_id = $1)
	  AND c.deleted = false
	ORDER BY COALESCE(lm.created_at, c.updated_at) DESC, c.chat_id DESC
`

const queryHasAccess = `
	SELECT EXISTS (
		SELECT 1 FROM chat
		WHERE chat_id = $1
		  AND deleted = false
		  AND (user_a_id = $2 OR user_b_id = $2)
	)
`

const queryListMessages = `
	SELECT
		m.message_id, m.chat_id, m.author_id,
		u.name, u.user_tag, m.body, m.related_to, m.created_at
	FROM chat_message AS m
	JOIN chat AS c ON c.chat_id = m.chat_id AND c.deleted = false
	JOIN "user" AS u ON u.user_id = m.author_id
	WHERE m.chat_id = $1
	  AND m.deleted = false
	  AND ($2 = 0 OR m.message_id < $2)
	ORDER BY m.message_id DESC
	LIMIT $3
`

const queryCreateMessage = `
	WITH created_message AS (
		INSERT INTO chat_message (chat_id, author_id, body, related_to)
		SELECT c.chat_id, $2, $3, $4
		FROM chat AS c
		WHERE c.chat_id = $1
		  AND c.deleted = false
		  AND (c.user_a_id = $2 OR c.user_b_id = $2)
		  AND (
			$4::integer IS NULL OR EXISTS (
				SELECT 1 FROM chat_message AS parent
				WHERE parent.message_id = $4
				  AND parent.chat_id = c.chat_id
				  AND parent.deleted = false
			)
		  )
		RETURNING message_id, chat_id, author_id, body, related_to, created_at
	), updated_chat AS (
		UPDATE chat SET updated_at = now()
		WHERE chat_id = $1 AND EXISTS (SELECT 1 FROM created_message)
	)
	SELECT
		m.message_id, m.chat_id, m.author_id,
		u.name, u.user_tag, m.body, m.related_to, m.created_at
	FROM created_message AS m
	JOIN "user" AS u ON u.user_id = m.author_id
`

func (r *Repository) CreateChat(ctx context.Context, userAID, userBID int) (domain.Chat, error) {
	requesterID := userAID
	if userAID > userBID {
		userAID, userBID = userBID, userAID
	}
	var chat domain.Chat
	err := r.pool.QueryRow(ctx, queryCreateChat, userAID, userBID, requesterID).Scan(
		&chat.ID, &chat.Name, &chat.CreatedAt, &chat.UpdatedAt,
		&chat.Participant.ID, &chat.Participant.Name, &chat.Participant.UserTag, &chat.Participant.AvatarURL,
	)
	if errors.Is(err, pgx.ErrNoRows) {
		return domain.Chat{}, domain.ErrUserNotFound
	}
	return chat, err
}

func (r *Repository) ListChats(ctx context.Context, userID int) ([]domain.Chat, error) {
	rows, err := r.pool.Query(ctx, queryListChats, userID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	chats := make([]domain.Chat, 0)
	for rows.Next() {
		var chat domain.Chat
		var hasLastMessage bool
		var lastMessage domain.MessageView
		if err := rows.Scan(
			&chat.ID, &chat.Name, &chat.CreatedAt, &chat.UpdatedAt,
			&chat.Participant.ID, &chat.Participant.Name, &chat.Participant.UserTag, &chat.Participant.AvatarURL,
			&hasLastMessage,
			&lastMessage.ID, &lastMessage.AuthorID, &lastMessage.AuthorName, &lastMessage.AuthorTag,
			&lastMessage.Body, &lastMessage.CreatedAt,
		); err != nil {
			return nil, err
		}
		if hasLastMessage {
			lastMessage.ChatID = chat.ID
			chat.LastMessage = &lastMessage
		}
		chats = append(chats, chat)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	return chats, nil
}

func (r *Repository) HasAccess(ctx context.Context, userID, chatID int) (bool, error) {
	var allowed bool
	err := r.pool.QueryRow(ctx, queryHasAccess, chatID, userID).Scan(&allowed)
	return allowed, err
}

func (r *Repository) ListMessages(ctx context.Context, userID, chatID, limit, beforeID int) ([]domain.MessageView, error) {
	allowed, err := r.HasAccess(ctx, userID, chatID)
	if err != nil {
		return nil, err
	}
	if !allowed {
		return nil, domain.ErrChatNotFound
	}

	rows, err := r.pool.Query(ctx, queryListMessages, chatID, beforeID, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	messages := make([]domain.MessageView, 0)
	for rows.Next() {
		message, err := scanMessage(rows)
		if err != nil {
			return nil, err
		}
		messages = append(messages, message)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	// The database returns newest first for efficient cursor pagination. The API
	// returns the selected page in chronological order for direct rendering.
	for left, right := 0, len(messages)-1; left < right; left, right = left+1, right-1 {
		messages[left], messages[right] = messages[right], messages[left]
	}
	return messages, nil
}

func (r *Repository) CreateMessage(ctx context.Context, chatID, authorID int, body string, relatedTo *int) (domain.MessageView, error) {
	var message domain.MessageView
	var nullableRelatedTo pgtype.Int4
	err := r.pool.QueryRow(ctx, queryCreateMessage, chatID, authorID, body, relatedTo).Scan(
		&message.ID, &message.ChatID, &message.AuthorID,
		&message.AuthorName, &message.AuthorTag, &message.Body, &nullableRelatedTo, &message.CreatedAt,
	)
	if err == nil {
		if nullableRelatedTo.Valid {
			id := int(nullableRelatedTo.Int32)
			message.RelatedTo = &id
		}
		return message, nil
	}
	if !errors.Is(err, pgx.ErrNoRows) {
		return domain.MessageView{}, err
	}

	allowed, accessErr := r.HasAccess(ctx, authorID, chatID)
	if accessErr != nil {
		return domain.MessageView{}, accessErr
	}
	if !allowed {
		return domain.MessageView{}, domain.ErrChatNotFound
	}
	if relatedTo != nil {
		return domain.MessageView{}, domain.ErrMessageNotFound
	}
	return domain.MessageView{}, domain.ErrChatNotFound
}

func scanMessage(row pgx.Row) (domain.MessageView, error) {
	var message domain.MessageView
	var relatedTo pgtype.Int4
	err := row.Scan(
		&message.ID, &message.ChatID, &message.AuthorID,
		&message.AuthorName, &message.AuthorTag, &message.Body, &relatedTo, &message.CreatedAt,
	)
	if relatedTo.Valid {
		id := int(relatedTo.Int32)
		message.RelatedTo = &id
	}
	return message, err
}
