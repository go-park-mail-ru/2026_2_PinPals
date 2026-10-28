package pinservice

import (
	"context"
	"errors"
	"strings"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"
)

var (
	ErrPinNotFound  = errors.New("pin not found")
	ErrTagNotFound  = errors.New("tag not found")
	ErrInvalidLimit = errors.New("limit must be greater than zero")
	ErrInvalidInput = errors.New("invalid input")
)

type CreatePinInput struct {
	ImageURL    string
	Name        string
	Description *string
	Tags        []string
}

type UpdatePinInput struct {
	Name        *string
	Description *string
}

type Pin struct {
	ID          int
	CreatorID   int
	ImageURL    string
	Name        string
	Description *string
	CreatedAt   time.Time
	UpdatedAt   time.Time
	DeletedAt   *time.Time
}

type Cursor struct {
	CreatedAt time.Time
	ID        int
}

type Tag struct {
	ID   int
	Name string
}

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
	input CreatePinInput,
) (Pin, error) {
	if strings.TrimSpace(input.Name) == "" ||
		strings.TrimSpace(input.ImageURL) == "" {
		return Pin{}, ErrInvalidInput
	}

	tx, err := r.pool.Begin(ctx)
	if err != nil {
		return Pin{}, err
	}
	defer func() {
		_ = tx.Rollback(ctx)
	}()

	var pin Pin

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
		return Pin{}, err
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
		err = tx.QueryRow(ctx, queryInsertTag, tagName).Scan(&tagID)
		if err != nil {
			return Pin{}, err
		}

		_, err = tx.Exec(ctx, queryAddTagToPin, pin.ID, tagID)
		if err != nil {
			return Pin{}, err
		}
	}

	if err := tx.Commit(ctx); err != nil {
		return Pin{}, err
	}

	return pin, nil
}

func (r *Repository) Update(
	ctx context.Context,
	pinID int,
	input UpdatePinInput,
	descriptionSet bool,
) (Pin, error) {
	var pin Pin

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
		return Pin{}, ErrPinNotFound
	}
	if err != nil {
		return Pin{}, err
	}

	return pin, nil
}

func (r *Repository) Delete(ctx context.Context, pinID int) error {
	var deletedID int

	err := r.pool.QueryRow(ctx, queryDeletePin, pinID).Scan(&deletedID)
	if errors.Is(err, pgx.ErrNoRows) {
		return ErrPinNotFound
	}
	return err
}

func (r *Repository) List(
	ctx context.Context,
	limit int,
	cursor *Cursor,
) ([]Pin, bool, error) {
	if limit < 1 {
		return nil, false, ErrInvalidLimit
	}

	query, args := buildListQuery(cursor, limit)

	rows, err := r.pool.Query(ctx, query, args...)
	if err != nil {
		return nil, false, err
	}
	defer rows.Close()

	results := make([]Pin, 0, limit+1)

	for rows.Next() {
		var pin Pin
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

func buildListQuery(cursor *Cursor, limit int) (string, []any) {
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
) (Pin, error) {
	var pin Pin

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
		return Pin{}, ErrPinNotFound
	}
	if err != nil {
		return Pin{}, err
	}

	return pin, nil
}

func (r *Repository) SearchTags(
	ctx context.Context,
	prefix string,
	limit int,
) ([]Tag, error) {
	if limit < 1 {
		return nil, ErrInvalidLimit
	}

	rows, err := r.pool.Query(ctx, querySearchTags, prefix, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	tags := make([]Tag, 0)
	for rows.Next() {
		var tag Tag
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

	err := r.pool.QueryRow(
		ctx,
		queryAddTagToPin,
		pinID,
		tagID,
	).Scan(&relationID)

	if errors.Is(err, pgx.ErrNoRows) {
		return ErrPinNotFound
	}
	if err != nil {
		return err
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
		return ErrTagNotFound
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
	if errors.Is(err, pgx.ErrNoRows) {
		var exists bool
		checkErr := r.pool.QueryRow(ctx, `
			SELECT EXISTS (
				SELECT 1 FROM pin
				WHERE pin_id = $1 AND deleted_at IS NULL
			)
		`, pinID).Scan(&exists)
		if checkErr != nil {
			return checkErr
		}
		if !exists {
			return ErrPinNotFound
		}
		return nil
	}
	if err != nil {
		return err
	}

	return nil
}

func isUniqueViolation(err error, constraintName string) bool {
	var pgErr *pgconn.PgError
	return errors.As(err, &pgErr) &&
		pgErr.Code == "23505" &&
		pgErr.ConstraintName == constraintName
}
