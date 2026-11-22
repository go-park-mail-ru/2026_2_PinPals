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

const (
	userTagConstraint          = "user_user_tag_key"
	subscriptionSelfConstraint = "subscription_no_self"
)

type queryRower interface {
	Query(context.Context, string, ...any) (pgx.Rows, error)
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

const queryCreateSubscription = `
	INSERT INTO subscription (subscriber_id, subscribee_id)
	SELECT $1, user_id
	FROM "user"
	WHERE user_id = $2
	  AND deleted_at IS NULL
	ON CONFLICT (subscribee_id, subscriber_id) DO UPDATE
	SET subscribee_id = EXCLUDED.subscribee_id
	RETURNING subscription_id
`

const queryDeleteSubscription = `
	DELETE FROM subscription
	WHERE subscriber_id = $1
	  AND subscribee_id = $2
	RETURNING subscription_id
`

const querySearchUsers = `
	SELECT user_id, name, user_tag, birth_date, description, avatar_url, created_at
	FROM "user"
	WHERE user_tag_tsv @@ websearch_to_tsquery('simple', $1)
	  AND deleted_at IS NULL
	ORDER BY
		ts_rank(user_tag_tsv, websearch_to_tsquery('simple', $1)) DESC,
		user_id DESC
	LIMIT $2 OFFSET $3
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

func (r *Repository) CreateSubscription(
	ctx context.Context,
	subscriberID int,
	subscribeeID int,
) error {
	var subscriptionID int

	err := r.pool.QueryRow(
		ctx,
		queryCreateSubscription,
		subscriberID,
		subscribeeID,
	).Scan(&subscriptionID)

	if errors.Is(err, pgx.ErrNoRows) {
		return domain.ErrUserNotFound
	}

	if err != nil {
		if isSelfSubscriptionViolation(err) {
			return domain.ErrValidation
		}

		return err
	}

	return nil
}

func (r *Repository) DeleteSubscription(
	ctx context.Context,
	subscriberID int,
	subscribeeID int,
) error {
	var subscriptionID int

	err := r.pool.QueryRow(
		ctx,
		queryDeleteSubscription,
		subscriberID,
		subscribeeID,
	).Scan(&subscriptionID)

	if errors.Is(err, pgx.ErrNoRows) {
		return domain.ErrSubscriptionNotFound
	}

	return err
}

func isSelfSubscriptionViolation(err error) bool {
	var pgErr *pgconn.PgError

	return errors.As(err, &pgErr) &&
		pgErr.Code == "23514" &&
		pgErr.ConstraintName == subscriptionSelfConstraint
}

func (r *Repository) SearchUsers(
	ctx context.Context,
	query string,
	limit int,
	offset int,
) ([]domain.User, error) {
	if limit < 1 || offset < 0 {
		return nil, domain.ErrValidation
	}

	rows, err := r.pool.Query(ctx, querySearchUsers, query, limit, offset)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	users := make([]domain.User, 0, limit)
	for rows.Next() {
		var user domain.User
		if err := rows.Scan(
			&user.ID,
			&user.Name,
			&user.UserTag,
			&user.BirthDate,
			&user.Description,
			&user.AvatarURL,
			&user.CreatedAt,
		); err != nil {
			return nil, err
		}
		users = append(users, user)
	}

	if err := rows.Err(); err != nil {
		return nil, err
	}

	return users, nil
}
