package repository

import (
	"context"
	"errors"
	"strings"

	"2026_2_PinPals/pin_service/domain"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"
)

// poolDB is the subset of pgxpool.Pool used by Repository. Keeping the
// dependency behind this interface lets repository tests exercise SQL and
// error mapping with pgxmock without a live PostgreSQL server.
type poolDB interface {
	Begin(context.Context) (pgx.Tx, error)
	Exec(context.Context, string, ...any) (pgconn.CommandTag, error)
	Query(context.Context, string, ...any) (pgx.Rows, error)
	QueryRow(context.Context, string, ...any) pgx.Row
}

type Repository struct {
	pool poolDB
}

func NewRepository(pool *pgxpool.Pool) *Repository {
	return &Repository{pool: pool}
}

const queryCreatePin = `
	INSERT INTO pin (
		creator_id, image_url, name, description
	)
	VALUES ($1, $2, $3, $4)
	RETURNING
		pin_id, creator_id, image_url, name, description,
		created_at
`

const queryUpdatePin = `
	UPDATE pin
	SET
		name = COALESCE($2::text, name),
		description = CASE
			WHEN $3::boolean THEN $4::text
			ELSE description
		END,
		updated_at = now()
	WHERE pin_id = $1
	  AND deleted_at IS NULL
	RETURNING
		pin_id, creator_id, image_url, name, description,
		created_at
`

const queryDeletePin = `
	UPDATE pin
	SET deleted_at = now()
	WHERE pin_id = $1
	  AND deleted_at IS NULL
	RETURNING pin_id
`

const queryListPinsFirstPage = `
	SELECT
		pin_id, creator_id, image_url, name, description,
		created_at
	FROM pin
	WHERE deleted_at IS NULL
	ORDER BY created_at DESC, pin_id DESC
	LIMIT $1
`

const queryListPinsAfterCursor = `
	SELECT
		pin_id, creator_id, image_url, name, description,
		created_at
	FROM pin
	WHERE deleted_at IS NULL
	  AND (created_at, pin_id) < ($1, $2)
	ORDER BY created_at DESC, pin_id DESC
	LIMIT $3
`

const queryGetPinByID = `
	SELECT
		pin_id, creator_id, image_url, name, description,
		created_at
	FROM pin
	WHERE pin_id = $1
	  AND deleted_at IS NULL
`

const queryPinExists = `
	SELECT EXISTS (
		SELECT 1 FROM pin
		WHERE pin_id = $1 AND deleted_at IS NULL
	)
`

const queryGetPinWithAuthorByID = `
	SELECT
		p.pin_id, p.creator_id, p.image_url, p.name, p.description,
		p.created_at,
		u.user_id, u.name, u.user_tag, u.avatar_url
	FROM pin AS p
	LEFT JOIN "user" AS u
		ON u.user_id = p.creator_id
	   AND u.deleted_at IS NULL
	WHERE p.pin_id = $1
	  AND p.deleted_at IS NULL
`

const querySearchPins = `
	SELECT
		pin_id, creator_id, image_url, name, description,
		created_at
	FROM pin
	WHERE name_tsv @@ websearch_to_tsquery('russian', $1)
	  AND deleted_at IS NULL
	ORDER BY
		ts_rank(name_tsv, websearch_to_tsquery('russian', $1)) DESC,
		pin_id DESC
	LIMIT $2 OFFSET $3
`

func (r *Repository) Create(
	ctx context.Context,
	creatorID int,
	input domain.CreatePinInput,
) (domain.Pin, error) {

	tx, err := r.pool.Begin(ctx)
	if err != nil {
		return domain.Pin{}, err
	}
	defer func() {
		_ = tx.Rollback(ctx)
	}()

	var pin domain.Pin

	err = tx.QueryRow(
		ctx,
		queryCreatePin,
		creatorID,
		input.ImageURL,
		input.Name,
		input.Description,
	).Scan(
		&pin.ID,
		&pin.CreatorID,
		&pin.ImageURL,
		&pin.Name,
		&pin.Description,
		&pin.CreatedAt,
	)
	if err != nil {
		return domain.Pin{}, err
	}

	seen := make(map[string]struct{}, len(input.Tags))

	for _, rawTag := range input.Tags {
		tagName := strings.TrimSpace(rawTag)
		if tagName == "" {
			continue
		}

		key := strings.ToLower(tagName)
		if _, exists := seen[key]; exists {
			continue
		}
		seen[key] = struct{}{}

		var tagID int
		if err := tx.QueryRow(ctx, queryInsertTag, tagName).Scan(&tagID); err != nil {
			return domain.Pin{}, err
		}

		if _, err := tx.Exec(ctx, queryAddTagToPin, pin.ID, tagID); err != nil {
			return domain.Pin{}, err
		}
	}

	if err := tx.Commit(ctx); err != nil {
		return domain.Pin{}, err
	}

	return pin, nil
}

func (r *Repository) Update(
	ctx context.Context,
	pinID int,
	input domain.UpdatePinInput,
	descriptionSet bool,
) (domain.Pin, error) {
	var pin domain.Pin

	err := r.pool.QueryRow(
		ctx,
		queryUpdatePin,
		pinID,
		input.Name,
		descriptionSet,
		input.Description,
	).Scan(
		&pin.ID,
		&pin.CreatorID,
		&pin.ImageURL,
		&pin.Name,
		&pin.Description,
		&pin.CreatedAt,
	)
	if errors.Is(err, pgx.ErrNoRows) {
		return domain.Pin{}, domain.ErrPinNotFound
	}
	if err != nil {
		return domain.Pin{}, err
	}

	return pin, nil
}

func (r *Repository) Delete(ctx context.Context, pinID int) error {
	var deletedID int

	err := r.pool.QueryRow(ctx, queryDeletePin, pinID).Scan(&deletedID)
	if errors.Is(err, pgx.ErrNoRows) {
		return domain.ErrPinNotFound
	}
	return err
}

func (r *Repository) List(
	ctx context.Context,
	limit int,
	cursor *domain.Cursor,
) ([]domain.Pin, bool, error) {

	query, args := buildListQuery(cursor, limit)

	rows, err := r.pool.Query(ctx, query, args...)
	if err != nil {
		return nil, false, err
	}
	defer rows.Close()

	results := make([]domain.Pin, 0, limit+1)

	for rows.Next() {
		var pin domain.Pin
		if err := rows.Scan(
			&pin.ID,
			&pin.CreatorID,
			&pin.ImageURL,
			&pin.Name,
			&pin.Description,
			&pin.CreatedAt,
		); err != nil {
			return nil, false, err
		}
		results = append(results, pin)
	}

	if err := rows.Err(); err != nil {
		return nil, false, err
	}

	hasNext := len(results) > limit
	if hasNext {
		results = results[:limit]
	}

	return results, hasNext, nil
}

func buildListQuery(cursor *domain.Cursor, limit int) (string, []any) {
	if cursor == nil {
		return queryListPinsFirstPage, []any{limit + 1}
	}

	return queryListPinsAfterCursor, []any{
		cursor.CreatedAt,
		cursor.ID,
		limit + 1,
	}
}

func (r *Repository) GetByID(
	ctx context.Context,
	pinID int,
) (domain.Pin, error) {
	var pin domain.Pin

	err := r.pool.QueryRow(ctx, queryGetPinByID, pinID).Scan(
		&pin.ID,
		&pin.CreatorID,
		&pin.ImageURL,
		&pin.Name,
		&pin.Description,
		&pin.CreatedAt,
	)
	if errors.Is(err, pgx.ErrNoRows) {
		return domain.Pin{}, domain.ErrPinNotFound
	}
	if err != nil {
		return domain.Pin{}, err
	}

	return pin, nil
}

func (r *Repository) GetWithAuthor(
	ctx context.Context,
	pinID int,
) (domain.PinWithAuthor, error) {
	var (
		result       domain.PinWithAuthor
		authorID     *int
		authorName   *string
		authorTag    *string
		authorAvatar *string
	)

	err := r.pool.QueryRow(ctx, queryGetPinWithAuthorByID, pinID).Scan(
		&result.ID,
		&result.CreatorID,
		&result.ImageURL,
		&result.Name,
		&result.Description,
		&result.CreatedAt,
		&authorID,
		&authorName,
		&authorTag,
		&authorAvatar,
	)
	if errors.Is(err, pgx.ErrNoRows) {
		return domain.PinWithAuthor{}, domain.ErrPinNotFound
	}
	if err != nil {
		return domain.PinWithAuthor{}, err
	}

	if authorID != nil && authorName != nil && authorTag != nil {
		result.Author = domain.Author{
			ID:        *authorID,
			Name:      *authorName,
			UserTag:   *authorTag,
			AvatarURL: authorAvatar,
		}
	}

	return result, nil
}

func (r *Repository) SearchPins(
	ctx context.Context,
	query string,
	limit int,
	offset int,
) ([]domain.Pin, error) {
	if limit < 1 || offset < 0 {
		return nil, domain.ErrValidation
	}

	rows, err := r.pool.Query(ctx, querySearchPins, query, limit, offset)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	pins := make([]domain.Pin, 0, limit)
	for rows.Next() {
		var pin domain.Pin
		if err := rows.Scan(
			&pin.ID,
			&pin.CreatorID,
			&pin.ImageURL,
			&pin.Name,
			&pin.Description,
			&pin.CreatedAt,
		); err != nil {
			return nil, err
		}
		pins = append(pins, pin)
	}

	if err := rows.Err(); err != nil {
		return nil, err
	}

	return pins, nil
}
