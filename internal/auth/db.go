package auth

import (
	"context"
	"errors"

	"2026_2_PinPals/internal/model"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"
)

var ErrUserNotFound = errors.New("user not found")
var ErrUserTagExists = errors.New("user tag already exists")

type Repository struct {
	pool *pgxpool.Pool
}

func NewRepository(pool *pgxpool.Pool) *Repository {
	return &Repository{pool: pool}
}

func (r *Repository) CreateUser(ctx context.Context, user model.RegisterInput, passwordHash string) (model.User, error) {
	const query = `
		INSERT INTO "user" (name, user_tag, age, description, avatar_url)
		VALUES ($1, $2, $3, $4, $5)
		RETURNING user_id, name, user_tag, age, description, avatar_url, deleted, created_at, updated_at, deleted_at
	`

	var result model.User
	err := r.pool.QueryRow(ctx, query,
		user.Name,
		user.UserTag,
		user.Age,
		user.Description,
		user.AvatarURL,
	).Scan(
		&result.ID,
		&result.Name,
		&result.UserTag,
		&result.Age,
		&result.Description,
		&result.AvatarURL,
		&result.Deleted,
		&result.CreatedAt,
		&result.UpdatedAt,
		&result.DeletedAt,
	)
	if err != nil {
		if isUniqueViolation(err) {
			return model.User{}, ErrUserTagExists
		}
		return model.User{}, err
	}

	_, err = r.pool.Exec(ctx, `INSERT INTO user_auth (user_id, password_hash) VALUES ($1, $2)`, result.ID, passwordHash)
	if err != nil {
		_, _ = r.pool.Exec(ctx, `DELETE FROM "user" WHERE user_id = $1`, result.ID)
		return model.User{}, err
	}

	return result, nil
}

func (r *Repository) GetUserWithPasswordHash(ctx context.Context, userTag string) (model.User, string, error) {
	const query = `
		SELECT u.user_id, u.name, u.user_tag, u.age, u.description, u.avatar_url,
		       u.deleted, u.created_at, u.updated_at, u.deleted_at, a.password_hash
		FROM "user" u
		JOIN user_auth a ON a.user_id = u.user_id
		WHERE u.user_tag = $1 AND u.deleted = false
	`

	var user model.User
	var passwordHash string
	err := r.pool.QueryRow(ctx, query, userTag).Scan(
		&user.ID,
		&user.Name,
		&user.UserTag,
		&user.Age,
		&user.Description,
		&user.AvatarURL,
		&user.Deleted,
		&user.CreatedAt,
		&user.UpdatedAt,
		&user.DeletedAt,
		&passwordHash,
	)
	if errors.Is(err, pgx.ErrNoRows) {
		return model.User{}, "", ErrUserNotFound
	}
	if err != nil {
		return model.User{}, "", err
	}
	return user, passwordHash, nil
}

func isUniqueViolation(err error) bool {
	var pgErr *pgconn.PgError
	return errors.As(err, &pgErr) && pgErr.Code == "23505"
}
