package pins

import (
	"context"
	"errors"

	"2026_2_PinPals/internal/model"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

var ErrPinNotFound = errors.New("pin not found")

type Repository struct {
	pool *pgxpool.Pool
}

func NewRepository(pool *pgxpool.Pool) *Repository {
	return &Repository{pool: pool}
}

func (r *Repository) Create(ctx context.Context, creatorID int, input model.CreatePinInput) (model.Pin, error) {
	const query = `
		INSERT INTO pin (creator_id, image_url, name, description)
		VALUES ($1, $2, $3, $4)
		RETURNING pin_id, creator_id, image_url, name, description, deleted, created_at, updated_at, deleted_at
	`

	var pin model.Pin
	err := r.pool.QueryRow(ctx, query, creatorID, input.ImageURL, input.Name, input.Description).Scan(
		&pin.ID,
		&pin.CreatorID,
		&pin.ImageURL,
		&pin.Name,
		&pin.Description,
		&pin.Deleted,
		&pin.CreatedAt,
		&pin.UpdatedAt,
		&pin.DeletedAt,
	)

	return pin, err
}

func (r *Repository) List(ctx context.Context, limit int, cursor *Cursor) ([]model.Pin, bool, error) {
	const baseQuery = `
		SELECT pin_id, creator_id, image_url, name, description, deleted, created_at, updated_at, deleted_at
		FROM pin
		WHERE deleted = false
	`

	const orderAndLimit = `
		ORDER BY created_at DESC, pin_id DESC
		LIMIT $1
	`

	var (
		rows    pgx.Rows
		err     error
		query   string
		results []model.Pin
	)

	if cursor == nil {
		query = baseQuery + orderAndLimit
		rows, err = r.pool.Query(ctx, query, limit+1)
	} else {
		query = baseQuery + `
			AND (created_at, pin_id) < ($1, $2)
		` + `
			ORDER BY created_at DESC, pin_id DESC
			LIMIT $3
		`
		rows, err = r.pool.Query(ctx, query, cursor.CreatedAt, cursor.ID, limit+1)
	}

	if err != nil {
		return nil, false, err
	}
	defer rows.Close()

	results = make([]model.Pin, 0, limit+1)

	for rows.Next() {
		var pin model.Pin
		if err := rows.Scan(
			&pin.ID,
			&pin.CreatorID,
			&pin.ImageURL,
			&pin.Name,
			&pin.Description,
			&pin.Deleted,
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
	return results, hasNext, nil
}

func (r *Repository) GetByID(ctx context.Context, id int) (model.Pin, error) {
	const query = `
		SELECT pin_id, creator_id, image_url, name, description, deleted, created_at, updated_at, deleted_at
		FROM pin
		WHERE pin_id = $1 AND deleted = false
	`

	var pin model.Pin
	err := r.pool.QueryRow(ctx, query, id).Scan(
		&pin.ID,
		&pin.CreatorID,
		&pin.ImageURL,
		&pin.Name,
		&pin.Description,
		&pin.Deleted,
		&pin.CreatedAt,
		&pin.UpdatedAt,
		&pin.DeletedAt,
	)
	if errors.Is(err, pgx.ErrNoRows) {
		return model.Pin{}, ErrPinNotFound
	}
	if err != nil {
		return model.Pin{}, err
	}
	return pin, nil
}
