package repository

import (
	"context"
	"errors"
	"strings"

	"2026_2_PinPals/pin_service/domain"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

type Repository struct {
	pool *pgxpool.Pool
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
		created_at, updated_at, deleted_at
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
		created_at, updated_at, deleted_at
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
		created_at, updated_at, deleted_at
	FROM pin
	WHERE deleted_at IS NULL
	ORDER BY created_at DESC, pin_id DESC
	LIMIT $1
`

const queryListPinsAfterCursor = `
	SELECT
		pin_id, creator_id, image_url, name, description,
		created_at, updated_at, deleted_at
	FROM pin
	WHERE deleted_at IS NULL
	  AND (created_at, pin_id) < ($1, $2)
	ORDER BY created_at DESC, pin_id DESC
	LIMIT $3
`

const queryGetPinByID = `
	SELECT
		pin_id, creator_id, image_url, name, description,
		created_at, updated_at, deleted_at
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

const queryTagExists = `
	SELECT EXISTS (
		SELECT 1 FROM tag WHERE tag_id = $1
	)
`

const querySearchTags = `
	SELECT tag_id, name
	FROM tag
	WHERE lower(name) LIKE lower($1) || '%'
	ORDER BY lower(name), tag_id
	LIMIT $2
`

const queryInsertTag = `
	INSERT INTO tag (name)
	VALUES ($1)
	ON CONFLICT (name) DO UPDATE
	SET name = EXCLUDED.name
	RETURNING tag_id
`

const queryAddTagToPin = `
	INSERT INTO pin_tag_relation (pin_id, tag_id)
	SELECT $1, tag_id
	FROM tag
	WHERE tag_id = $2
	  AND EXISTS (
		  SELECT 1 FROM pin
		  WHERE pin_id = $1
		    AND deleted_at IS NULL
	  )
	ON CONFLICT (tag_id, pin_id) DO NOTHING
	RETURNING tag_relation_id
`

const queryRemoveTagFromPin = `
	DELETE FROM pin_tag_relation AS ptr
	USING pin AS p
	WHERE ptr.pin_id = p.pin_id
	  AND ptr.pin_id = $1
	  AND ptr.tag_id = $2
	  AND p.deleted_at IS NULL
	RETURNING ptr.tag_relation_id
`

const queryAddLike = `
	INSERT INTO pin_like (liker_id, pin_id)
	SELECT $1, pin_id
	FROM pin
	WHERE pin_id = $2
	  AND deleted_at IS NULL
	ON CONFLICT (liker_id, pin_id) DO NOTHING
	RETURNING pin_like_id
`

func (r *Repository) Create(
	ctx context.Context,
	creatorID int,
	input domain.CreatePinInput,
) (domain.Pin, error) {
	if strings.TrimSpace(input.Name) == "" ||
		strings.TrimSpace(input.ImageURL) == "" {
		return domain.Pin{}, domain.ErrValidation
	}

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
		&pin.UpdatedAt,
		&pin.DeletedAt,
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
		&pin.UpdatedAt,
		&pin.DeletedAt,
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
	if limit < 1 {
		return nil, false, domain.ErrValidation
	}

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
			&pin.UpdatedAt,
			&pin.DeletedAt,
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
		&pin.UpdatedAt,
		&pin.DeletedAt,
	)
	if errors.Is(err, pgx.ErrNoRows) {
		return domain.Pin{}, domain.ErrPinNotFound
	}
	if err != nil {
		return domain.Pin{}, err
	}

	return pin, nil
}

func (r *Repository) SearchTags(
	ctx context.Context,
	prefix string,
	limit int,
) ([]domain.Tag, error) {
	if limit < 1 {
		return nil, domain.ErrValidation
	}

	rows, err := r.pool.Query(ctx, querySearchTags, prefix, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	tags := make([]domain.Tag, 0)
	for rows.Next() {
		var tag domain.Tag
		if err := rows.Scan(&tag.ID, &tag.Name); err != nil {
			return nil, err
		}
		tags = append(tags, tag)
	}

	if err := rows.Err(); err != nil {
		return nil, err
	}
	return tags, nil
}

func (r *Repository) AddTag(
	ctx context.Context,
	pinID int,
	tagID int,
) error {
	var relationID int

	err := r.pool.QueryRow(ctx, queryAddTagToPin, pinID, tagID).Scan(&relationID)
	if err == nil {
		return nil
	}
	if !errors.Is(err, pgx.ErrNoRows) {
		return err
	}

	var pinExists bool
	if err := r.pool.QueryRow(ctx, queryPinExists, pinID).Scan(&pinExists); err != nil {
		return err
	}
	if !pinExists {
		return domain.ErrPinNotFound
	}

	var tagExists bool
	if err := r.pool.QueryRow(ctx, queryTagExists, tagID).Scan(&tagExists); err != nil {
		return err
	}
	if !tagExists {
		return domain.ErrTagNotFound
	}

	return nil
}

func (r *Repository) RemoveTag(
	ctx context.Context,
	pinID int,
	tagID int,
) error {
	var relationID int

	err := r.pool.QueryRow(
		ctx,
		queryRemoveTagFromPin,
		pinID,
		tagID,
	).Scan(&relationID)

	if errors.Is(err, pgx.ErrNoRows) {
		return domain.ErrTagNotFound
	}
	return err
}

func (r *Repository) AddLike(
	ctx context.Context,
	userID int,
	pinID int,
) error {
	var likeID int

	err := r.pool.QueryRow(ctx, queryAddLike, userID, pinID).Scan(&likeID)
	if err == nil {
		return nil
	}
	if !errors.Is(err, pgx.ErrNoRows) {
		return err
	}

	var exists bool
	if err := r.pool.QueryRow(ctx, queryPinExists, pinID).Scan(&exists); err != nil {
		return err
	}
	if !exists {
		return domain.ErrPinNotFound
	}

	return nil
}
