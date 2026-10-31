package repository

import (
	"context"
	"errors"

	"2026_2_PinPals/user_service/domain"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"
)

const userTagConstraint = "user_user_tag_key"

type Repository struct {
	pool *pgxpool.Pool
}

func NewRepository(pool *pgxpool.Pool) *Repository {
	return &Repository{pool: pool}
}

const queryUpdateProfile = `
	UPDATE "user"
	SET
		name = COALESCE($2::text, name),
		user_tag = COALESCE($3::text, user_tag),
		description = CASE
			WHEN $4::boolean THEN $5::text
			ELSE description
		END,
		avatar_url = CASE
			WHEN $6::boolean THEN $7::text
			ELSE avatar_url
		END,
		updated_at = now()
	WHERE user_id = $1
	  AND deleted_at IS NULL
	RETURNING
		user_id, name, user_tag, birth_date, description,
		avatar_url, created_at, updated_at, deleted_at
`

const queryDeleteProfile = `
	UPDATE "user"
	SET deleted_at = now()
	WHERE user_id = $1
	  AND deleted_at IS NULL
	RETURNING user_id
`

func (r *Repository) UpdateProfile(
	ctx context.Context,
	userID int,
	input domain.UpdateProfileInput,
) (domain.User, error) {
	var user domain.User

	err := r.pool.QueryRow(
		ctx,
		queryUpdateProfile,
		userID,
		input.Name,
		input.UserTag,
		input.DescriptionSet,
		input.Description,
		input.AvatarURLSet,
		input.AvatarURL,
	).Scan(
		&user.ID,
		&user.Name,
		&user.UserTag,
		&user.BirthDate,
		&user.Description,
		&user.AvatarURL,
		&user.CreatedAt,
		&user.UpdatedAt,
		&user.DeletedAt,
	)
	if errors.Is(err, pgx.ErrNoRows) {
		return domain.User{}, domain.ErrUserNotFound
	}
	if err != nil {
		if isUserTagUniqueViolation(err) {
			return domain.User{}, domain.ErrUserTagExists
		}
		return domain.User{}, err
	}

	return user, nil
}

func (r *Repository) DeleteProfile(ctx context.Context, userID int) error {
	var deletedID int

	err := r.pool.QueryRow(ctx, queryDeleteProfile, userID).Scan(&deletedID)
	if errors.Is(err, pgx.ErrNoRows) {
		return domain.ErrUserNotFound
	}
	return err
}

func isUserTagUniqueViolation(err error) bool {
	var pgErr *pgconn.PgError

	return errors.As(err, &pgErr) &&
		pgErr.Code == "23505" &&
		pgErr.ConstraintName == userTagConstraint
}
