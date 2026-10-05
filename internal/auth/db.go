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
	var result model.User

	err := pgx.BeginFunc(ctx, r.pool, func(tx pgx.Tx) error {
		const query = `
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
        `

		err := tx.QueryRow(
			ctx,
			query,
			user.Name,
			user.UserTag,
			user.BirthDate,
			user.Description,
			user.AvatarURL,
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
			return err
		}

		_, err = tx.Exec(
			ctx,
			`INSERT INTO password (user_id, password_hash)
             VALUES ($1, $2)`,
			result.ID,
			passwordHash,
		)

		return err
	})

	if err != nil {
		if isUniqueViolation(err) {
			return model.User{}, ErrUserTagExists
		}

		return model.User{}, err
	}

	return result, nil
}

func (r *Repository) GetUserWithPasswordHash(ctx context.Context, userTag string) (model.User, string, error) {
	const query = `
		SELECT u.user_id, u.name, u.user_tag, u.birth_date, u.description, u.avatar_url,
		       u.deleted, u.created_at, u.updated_at, u.deleted_at, a.password_hash
		FROM "user" u
		JOIN password a ON a.user_id = u.user_id
		WHERE u.user_tag = $1 AND u.deleted = false
	`

	var user model.User
	var passwordHash string
	err := r.pool.QueryRow(ctx, query, userTag).Scan(
		&user.ID,
		&user.Name,
		&user.UserTag,
		&user.BirthDate,
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
