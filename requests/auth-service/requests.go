package authservice

import (
	"context"
	"errors"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"
)

type RegisterInput struct {
	Name        string
	UserTag     string
	BirthDate   time.Time
	Description *string
	AvatarURL   *string
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

var (
	ErrUserTagExists = errors.New("user tag already exists")
	ErrUserNotFound  = errors.New("user not found")
)

const queryCreateUser = `
    WITH inserted_user AS (
        INSERT INTO "user" (
            name,
            user_tag,
            birth_date,
            description,
            avatar_url
        )
        VALUES ($1, $2, $3, $4, $5)
        RETURNING
            user_id,
            name,
            user_tag,
            birth_date,
            description,
            avatar_url,
            created_at,
            updated_at,
            deleted_at
    ),
    inserted_password AS (
        INSERT INTO "password" (user_id, password_hash)
        SELECT user_id, $6
        FROM inserted_user
        RETURNING user_id
    )
    SELECT
        u.user_id,
        u.name,
        u.user_tag,
        u.birth_date,
        u.description,
        u.avatar_url,
        u.created_at,
        u.updated_at,
        u.deleted_at
    FROM inserted_user AS u
    JOIN inserted_password AS p USING (user_id)
`

const queryGetUserWithPasswordHash = `
    SELECT
        u.user_id,
        u.name,
        u.user_tag,
        u.birth_date,
        u.description,
        u.avatar_url,
        u.created_at,
        u.updated_at,
        u.deleted_at,
        p.password_hash
    FROM "user" AS u
    JOIN "password" AS p ON p.user_id = u.user_id
    WHERE u.user_tag = $1
      AND u.deleted_at IS NULL
`

type Repository struct {
	pool *pgxpool.Pool
}

func NewRepository(pool *pgxpool.Pool) *Repository {
	return &Repository{pool: pool}
}

func (r *Repository) CreateUser(
	ctx context.Context,
	input RegisterInput,
	passwordHash string,) (User, error) {
	var result User

	err := r.pool.QueryRow(
		ctx,
		queryCreateUser,
		input.Name,
		input.UserTag,
		input.BirthDate,
		input.Description,
		input.AvatarURL,
		passwordHash,
	).Scan(
		&result.ID,
		&result.Name,
		&result.UserTag,
		&result.BirthDate,
		&result.Description,
		&result.AvatarURL,
		&result.CreatedAt,
		&result.UpdatedAt,
		&result.DeletedAt,
	)
	if err != nil {
		if isUserTagUniqueViolation(err) {
			return User{}, ErrUserTagExists
		}
		return User{}, err
	}

	return result, nil
}

func (r *Repository) GetUserWithPasswordHash(
	ctx context.Context,
	userTag string,) (User, string, error) {
	var user User
	var passwordHash string

	err := r.pool.QueryRow(
		ctx,
		queryGetUserWithPasswordHash,
		userTag,
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
		&passwordHash,
	)
	if errors.Is(err, pgx.ErrNoRows) {
		return User{}, "", ErrUserNotFound
	}
	if err != nil {
		return User{}, "", err
	}

	return user, passwordHash, nil
}

func isUserTagUniqueViolation(err error) bool {
	var pgErr *pgconn.PgError

	return errors.As(err, &pgErr) &&
		pgErr.Code == "23505" &&
		pgErr.ConstraintName == "user_user_tag_key"
}
