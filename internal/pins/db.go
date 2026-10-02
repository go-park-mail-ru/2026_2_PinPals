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

func (r *Repository) List(ctx context.Context, limit, offset int) ([]model.Pin, error) {
	const query = `
		SELECT pin_id, creator_id, image_url, name, description, deleted, created_at, updated_at, deleted_at
		FROM pin
		WHERE deleted = false
		ORDER BY created_at DESC, pin_id DESC
		LIMIT $1 OFFSET $2
	`

	rows, err := r.pool.Query(ctx, query, limit, offset)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	pins := make([]model.Pin, 0, limit)
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
			return nil, err
		}
		pins = append(pins, pin)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	return pins, nil
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
