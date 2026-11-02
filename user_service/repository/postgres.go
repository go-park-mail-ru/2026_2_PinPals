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

type Repository struct {
	pool *pgxpool.Pool
}

func NewRepository(pool *pgxpool.Pool) *Repository {
	return &Repository{pool: pool}
}

const queryDeleteProfile = `
	UPDATE "user"
	SET deleted_at = now()
	WHERE user_id = $1
	  AND deleted_at IS NULL
	RETURNING user_id
`

func buildUpdateProfileQuery(
	userID int,
	input domain.UpdateProfileInput,
) (string, []any, error) {
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

	if len(setClauses) == 0 {
		return "", nil, domain.ErrValidation
	}

	query := `UPDATE "user"
SET ` + strings.Join(setClauses, ", ") + `,
    updated_at = now()
WHERE user_id = $1
  AND deleted_at IS NULL
RETURNING
    user_id, name, user_tag, birth_date, description,
    avatar_url, created_at`

	return query, args, nil
}

func (r *Repository) UpdateProfile(
	ctx context.Context,
	userID int,
	input domain.UpdateProfileInput,
) (domain.User, error) {
	var user domain.User

	query, args, err := buildUpdateProfileQuery(userID, input)
	if err != nil {
		return domain.User{}, err
	}

	err = r.pool.QueryRow(ctx, query, args...).Scan(
		&user.ID,
		&user.Name,
		&user.UserTag,
		&user.BirthDate,
		&user.Description,
		&user.AvatarURL,
		&user.CreatedAt,
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
