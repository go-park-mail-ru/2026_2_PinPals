package repository

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"2026_2_PinPals/user_service/domain"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"
)

const userTagConstraint = "user_user_tag_key"

type queryRower interface {
	QueryRow(context.Context, string, ...any) pgx.Row
}

type Repository struct {
	pool queryRower
}

func NewRepository(pool *pgxpool.Pool) *Repository {
	return &Repository{pool: pool}
}

const queryGetProfile = `
	SELECT user_id, name, user_tag, birth_date, description, avatar_url, created_at
	FROM "user"
	WHERE user_id = $1
	  AND deleted_at IS NULL
`

const queryDeleteProfile = `
	UPDATE "user"
	SET deleted_at = now()
	WHERE user_id = $1
	  AND deleted_at IS NULL
	RETURNING user_id
`

func (r *Repository) GetProfile(ctx context.Context, userID int) (*domain.User, error) {
	user := &domain.User{}

	err := r.pool.QueryRow(ctx, queryGetProfile, userID).Scan(
		&user.ID,
		&user.Name,
		&user.UserTag,
		&user.BirthDate,
		&user.Description,
		&user.AvatarURL,
		&user.CreatedAt,
	)

	if errors.Is(err, pgx.ErrNoRows) {
		return nil, domain.ErrUserNotFound
	}

	if err != nil {
		return nil, err
	}

	return user, nil
}

func buildUpdateProfileQuery(
	userID int,
	input *domain.UpdateProfileInput,
) (string, []any) {
	args := []any{userID}
	setClauses := make([]string, 0, 4)

	addSet := func(column string, value any) {
		args = append(args, value)

		setClauses = append(
			setClauses,
			fmt.Sprintf("%s = $%d", column, len(args)),
		)
	}

	if input.Name != nil {
		addSet("name", *input.Name)
	}

	if input.UserTag != nil {
		addSet("user_tag", *input.UserTag)
	}

	if input.DescriptionSet {
		var description any

		if input.Description != nil {
			description = *input.Description
		}

		addSet("description", description)
	}

	if input.AvatarURLSet {
		var avatarURL any

		if input.AvatarURL != nil {
			avatarURL = *input.AvatarURL
		}

		addSet("avatar_url", avatarURL)
	}

	query := `UPDATE "user"
SET ` + strings.Join(setClauses, ", ") + `,
    updated_at = now()
WHERE user_id = $1
  AND deleted_at IS NULL
RETURNING
    user_id, name, user_tag, birth_date, description,
    avatar_url, created_at`

	return query, args
}

func (r *Repository) UpdateProfile(
	ctx context.Context,
	userID int,
	input *domain.UpdateProfileInput,
) (*domain.User, error) {
	user := &domain.User{}

	query, args := buildUpdateProfileQuery(userID, input)

	err := r.pool.QueryRow(ctx, query, args...).Scan(
		&user.ID,
		&user.Name,
		&user.UserTag,
		&user.BirthDate,
		&user.Description,
		&user.AvatarURL,
		&user.CreatedAt,
	)

	if errors.Is(err, pgx.ErrNoRows) {
		return nil, domain.ErrUserNotFound
	}

	if err != nil {
		if isUserTagUniqueViolation(err) {
			return nil, domain.ErrUserTagExists
		}

		return nil, err
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
