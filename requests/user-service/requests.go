package userservice

import (
	"context"
	"errors"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"
)

var (
	ErrUserNotFound  = errors.New("user not found")
	ErrUserTagExists = errors.New("user tag already exists")
	ErrInvalidInput  = errors.New("invalid input")
)

type UpdateProfileInput struct {
	Name           *string
	UserTag        *string
	Description    *string
	DescriptionSet bool
	AvatarURL      *string
	AvatarURLSet   bool
}

type User struct {
	ID          int
	Name        string
	UserTag     string
	BirthDate   time.Time
	Description *string
	AvatarURL   *string
	CreatedAt   time.Time
	UpdatedAt   time.Time
	DeletedAt   *time.Time
}

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
	input UpdateProfileInput,
) (User, error) {
	var user User

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
		return User{}, ErrUserNotFound
	}

	if err != nil {
		if isUserTagUniqueViolation(err) {
			return User{}, ErrUserTagExists
		}
		return User{}, err
	}

	return user, nil
}

func (r *Repository) DeleteProfile(
	ctx context.Context,
	userID int,
) error {
	var deletedID int

	err := r.pool.QueryRow(
		ctx,
		queryDeleteProfile,
		userID,
	).Scan(&deletedID)

	if errors.Is(err, pgx.ErrNoRows) {
		return ErrUserNotFound
	}

	return err
}

func isUserTagUniqueViolation(err error) bool {
	var pgErr *pgconn.PgError

	return errors.As(err, &pgErr) &&
		pgErr.Code == "23505" &&
		pgErr.ConstraintName == "user_user_tag_key"
}
