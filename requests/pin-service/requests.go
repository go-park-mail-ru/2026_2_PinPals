package pinservice

import (
	"context"
	"errors"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

type CreatePinInput struct {
	ImageURL    string
	Name        string
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

var (
	ErrPinNotFound  = errors.New("pin not found")
	ErrInvalidLimit = errors.New("limit must be greater than zero")
)

const queryCreatePin = `
    INSERT INTO pin (
        creator_id,
        image_url,
        name,
        description
    )
    VALUES ($1, $2, $3, $4)
    RETURNING
        pin_id,
        creator_id,
        image_url,
        name,
        description,
        created_at,
        updated_at,
        deleted_at
`

const queryListPinsFirstPage = `
    SELECT
        pin_id,
        creator_id,
        image_url,
        name,
        description,
        created_at,
        updated_at,
        deleted_at
    FROM pin
    WHERE deleted_at IS NULL
    ORDER BY created_at DESC, pin_id DESC
    LIMIT $1
`

const queryListPinsAfterCursor = `
    SELECT
        pin_id,
        creator_id,
        image_url,
        name,
        description,
        created_at,
        updated_at,
        deleted_at
    FROM pin
    WHERE deleted_at IS NULL
      AND (created_at, pin_id) < ($1, $2)
    ORDER BY created_at DESC, pin_id DESC
    LIMIT $3
`

const queryGetPinByID = `
    SELECT
        pin_id,
        creator_id,
        image_url,
        name,
        description,
        created_at,
        updated_at,
        deleted_at
    FROM pin
    WHERE pin_id = $1
      AND deleted_at IS NULL
`

type Repository struct {
	pool *pgxpool.Pool
}

func NewRepository(pool *pgxpool.Pool) *Repository {
	return &Repository{pool: pool}
}

func (r *Repository) Create(
	ctx context.Context,
	creatorID int,
	input CreatePinInput,) (Pin, error) {
	var pin Pin

	err := r.pool.QueryRow(
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

	return pin, nil
}

func (r *Repository) List(
	ctx context.Context,
	limit int,
	cursor *Cursor,) ([]Pin, bool, error) {
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

func buildListQuery(
	cursor *Cursor,
	limit int,) (string, []any) {
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
	id int,) (Pin, error) {
	var pin Pin

	err := r.pool.QueryRow(
		ctx,
		queryGetPinByID,
		id,
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
