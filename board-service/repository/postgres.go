package repository

import (
	"context"
	"errors"

	"2026_2_PinPals/board-service/domain"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

type Repository struct {
	pool *pgxpool.Pool
}

func NewRepository(pool *pgxpool.Pool) *Repository {
	return &Repository{pool: pool}
}

const queryCreateDesk = `
	INSERT INTO desk (
		creator_id,
		name,
		description,
		entry_status,
		avatar_img_url
	)
	VALUES ($1, $2, $3, $4, $5)
	RETURNING
		desk_id, creator_id, name, description,
		avatar_img_url, entry_status,
		created_at, updated_at, deleted_at
`

const queryListOwnDesks = `
	SELECT
		desk_id, creator_id, name, description,
		avatar_img_url, entry_status,
		created_at, updated_at, deleted_at
	FROM desk
	WHERE creator_id = $1
	  AND deleted_at IS NULL
	ORDER BY created_at DESC, desk_id DESC
`

const queryListPublicDesks = `
	SELECT
		desk_id, creator_id, name, description,
		avatar_img_url, entry_status,
		created_at, updated_at, deleted_at
	FROM desk
	WHERE creator_id = $1
	  AND entry_status = 0
	  AND deleted_at IS NULL
	ORDER BY created_at DESC, desk_id DESC
	LIMIT $2
`

const queryGetDeskByID = `
	SELECT
		desk_id, creator_id, name, description,
		avatar_img_url, entry_status,
		created_at, updated_at, deleted_at
	FROM desk
	WHERE desk_id = $1
	  AND deleted_at IS NULL
`

const queryUpdateDesk = `
	UPDATE desk
	SET
		name = COALESCE($2::text, name),
		description = CASE
			WHEN $3::boolean THEN $4::text
			ELSE description
		END,
		entry_status = COALESCE($5::smallint, entry_status),
		avatar_img_url = CASE
			WHEN $6::boolean THEN $7::text
			ELSE avatar_img_url
		END,
		updated_at = now()
	WHERE desk_id = $1
	  AND deleted_at IS NULL
	RETURNING
		desk_id, creator_id, name, description,
		avatar_img_url, entry_status,
		created_at, updated_at, deleted_at
`

const queryDeleteDesk = `
	UPDATE desk
	SET deleted_at = now()
	WHERE desk_id = $1
	  AND deleted_at IS NULL
	RETURNING desk_id
`

const queryAddPinToDesk = `
	INSERT INTO pin_desk_relation (desk_id, pin_id)
	SELECT $1, p.pin_id
	FROM pin AS p
	JOIN desk AS d ON d.desk_id = $1
	WHERE p.pin_id = $2
	  AND p.deleted_at IS NULL
	  AND d.deleted_at IS NULL
	  AND d.creator_id = $3
	ON CONFLICT (desk_id, pin_id) DO NOTHING
	RETURNING desk_relation_id
`

const queryRemovePinFromDesk = `
	DELETE FROM pin_desk_relation AS pdr
	USING desk AS d
	WHERE pdr.desk_id = d.desk_id
	  AND pdr.desk_id = $1
	  AND pdr.pin_id = $2
	  AND d.deleted_at IS NULL
	  AND d.creator_id = $3
	RETURNING pdr.desk_relation_id
`

const queryGetFirstPinFromDesk = `
	SELECT
		p.pin_id, p.creator_id, p.image_url, p.name,
		p.description, p.created_at, p.updated_at, p.deleted_at
	FROM pin_desk_relation AS pdr
	JOIN pin AS p ON p.pin_id = pdr.pin_id
	JOIN desk AS d ON d.desk_id = pdr.desk_id
	WHERE d.desk_id = $1
	  AND d.deleted_at IS NULL
	  AND p.deleted_at IS NULL
	ORDER BY pdr.created_at ASC, p.pin_id ASC
	LIMIT 1
`

const queryGetDeskPins = `
	SELECT
		p.pin_id, p.creator_id, p.image_url, p.name,
		p.description, p.created_at, p.updated_at, p.deleted_at
	FROM pin_desk_relation AS pdr
	JOIN pin AS p ON p.pin_id = pdr.pin_id
	WHERE pdr.desk_id = $1
	  AND p.deleted_at IS NULL
	ORDER BY pdr.created_at DESC, p.pin_id DESC
`

const queryDeskOwnedBy = `
	SELECT EXISTS (
		SELECT 1 FROM desk
		WHERE desk_id = $1
		  AND creator_id = $2
		  AND deleted_at IS NULL
	)
`

const queryPinExists = `
	SELECT EXISTS (
		SELECT 1 FROM pin
		WHERE pin_id = $1 AND deleted_at IS NULL
	)
`

func scanDesk(row pgx.Row) (domain.Desk, error) {
	var desk domain.Desk
	err := row.Scan(
		&desk.ID,
		&desk.CreatorID,
		&desk.Name,
		&desk.Description,
		&desk.AvatarImgURL,
		&desk.EntryStatus,
		&desk.CreatedAt,
		&desk.UpdatedAt,
		&desk.DeletedAt,
	)
	return desk, err
}

func scanPin(row pgx.Row) (domain.Pin, error) {
	var pin domain.Pin
	err := row.Scan(
		&pin.ID,
		&pin.CreatorID,
		&pin.ImageURL,
		&pin.Name,
		&pin.Description,
		&pin.CreatedAt,
		&pin.UpdatedAt,
		&pin.DeletedAt,
	)
	return pin, err
}

func (r *Repository) Create(
	ctx context.Context,
	creatorID int,
	input domain.CreateDeskInput,
) (domain.Desk, error) {
	if input.EntryStatus < 0 || input.EntryStatus > 2 {
		return domain.Desk{}, domain.ErrValidation
	}

	name := input.Name
	if name == "" {
		name = "Untitled desk"
	}

	desk, err := scanDesk(r.pool.QueryRow(
		ctx,
		queryCreateDesk,
		creatorID,
		name,
		input.Description,
		input.EntryStatus,
		input.AvatarImgURL,
	))
	if err != nil {
		return domain.Desk{}, err
	}

	return desk, nil
}

func (r *Repository) ListOwn(
	ctx context.Context,
	creatorID int,
) ([]domain.Desk, error) {
	rows, err := r.pool.Query(ctx, queryListOwnDesks, creatorID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	return collectDesks(rows)
}

func (r *Repository) ListPublic(
	ctx context.Context,
	requesterID int,
	limit int,
) ([]domain.Desk, error) {
	if limit < 1 {
		return nil, domain.ErrValidation
	}

	rows, err := r.pool.Query(ctx, queryListPublicDesks, requesterID, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	return collectDesks(rows)
}

func collectDesks(rows pgx.Rows) ([]domain.Desk, error) {
	desks := make([]domain.Desk, 0)

	for rows.Next() {
		desk, err := scanDesk(rows)
		if err != nil {
			return nil, err
		}
		desks = append(desks, desk)
	}

	if err := rows.Err(); err != nil {
		return nil, err
	}

	return desks, nil
}

func (r *Repository) GetByID(
	ctx context.Context,
	deskID int,
) (domain.Desk, error) {
	desk, err := scanDesk(r.pool.QueryRow(ctx, queryGetDeskByID, deskID))
	if errors.Is(err, pgx.ErrNoRows) {
		return domain.Desk{}, domain.ErrDeskNotFound
	}
	if err != nil {
		return domain.Desk{}, err
	}

	return desk, nil
}

func (r *Repository) GetDeskWithPins(
	ctx context.Context,
	deskID int,
) (domain.DeskWithPins, error) {
	desk, err := r.GetByID(ctx, deskID)
	if err != nil {
		return domain.DeskWithPins{}, err
	}

	rows, err := r.pool.Query(ctx, queryGetDeskPins, deskID)
	if err != nil {
		return domain.DeskWithPins{}, err
	}
	defer rows.Close()

	result := domain.DeskWithPins{
		Desk: desk,
		Pins: make([]domain.Pin, 0),
	}

	for rows.Next() {
		pin, err := scanPin(rows)
		if err != nil {
			return domain.DeskWithPins{}, err
		}
		result.Pins = append(result.Pins, pin)
	}

	if err := rows.Err(); err != nil {
		return domain.DeskWithPins{}, err
	}

	return result, nil
}

func (r *Repository) Update(
	ctx context.Context,
	deskID int,
	input domain.UpdateDeskInput,
) (domain.Desk, error) {
	if input.EntryStatus != nil &&
		(*input.EntryStatus < 0 || *input.EntryStatus > 2) {
		return domain.Desk{}, domain.ErrValidation
	}

	desk, err := scanDesk(r.pool.QueryRow(
		ctx,
		queryUpdateDesk,
		deskID,
		input.Name,
		input.DescriptionSet,
		input.Description,
		input.EntryStatus,
		input.AvatarImgURLSet,
		input.AvatarImgURL,
	))
	if errors.Is(err, pgx.ErrNoRows) {
		return domain.Desk{}, domain.ErrDeskNotFound
	}
	if err != nil {
		return domain.Desk{}, err
	}

	return desk, nil
}

func (r *Repository) Delete(ctx context.Context, deskID int) error {
	var deletedID int

	err := r.pool.QueryRow(ctx, queryDeleteDesk, deskID).Scan(&deletedID)
	if errors.Is(err, pgx.ErrNoRows) {
		return domain.ErrDeskNotFound
	}
	return err
}

func (r *Repository) AddPin(
	ctx context.Context,
	creatorID int,
	deskID int,
	pinID int,
) error {
	var relationID int

	err := r.pool.QueryRow(
		ctx,
		queryAddPinToDesk,
		deskID,
		pinID,
		creatorID,
	).Scan(&relationID)
	if err == nil {
		return nil
	}
	if !errors.Is(err, pgx.ErrNoRows) {
		return err
	}

	var owned bool
	if err := r.pool.QueryRow(ctx, queryDeskOwnedBy, deskID, creatorID).Scan(&owned); err != nil {
		return err
	}
	if !owned {
		return domain.ErrDeskNotFound
	}

	var pinExists bool
	if err := r.pool.QueryRow(ctx, queryPinExists, pinID).Scan(&pinExists); err != nil {
		return err
	}
	if !pinExists {
		return domain.ErrPinNotFound
	}

	return nil
}

func (r *Repository) RemovePin(
	ctx context.Context,
	creatorID int,
	deskID int,
	pinID int,
) error {
	var relationID int

	err := r.pool.QueryRow(
		ctx,
		queryRemovePinFromDesk,
		deskID,
		pinID,
		creatorID,
	).Scan(&relationID)

	if errors.Is(err, pgx.ErrNoRows) {
		return domain.ErrPinNotFound
	}
	return err
}

func (r *Repository) GetFirstPin(
	ctx context.Context,
	deskID int,
) (domain.Pin, error) {
	pin, err := scanPin(r.pool.QueryRow(ctx, queryGetFirstPinFromDesk, deskID))
	if errors.Is(err, pgx.ErrNoRows) {
		return domain.Pin{}, domain.ErrPinNotFound
	}
	if err != nil {
		return domain.Pin{}, err
	}

	return pin, nil
}
