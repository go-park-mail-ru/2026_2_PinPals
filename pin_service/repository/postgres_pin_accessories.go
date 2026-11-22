package repository

import (
	"context"
	"errors"

	"2026_2_PinPals/pin_service/domain"

	"github.com/jackc/pgx/v5"
)

const queryTagExists = `
	SELECT EXISTS (
		SELECT 1 FROM tag WHERE tag_id = $1
	)
`

const querySearchTags = `
	SELECT tag_id, name
	FROM tag
	WHERE lower(name) LIKE lower($1) || '%'
	ORDER BY lower(name), tag_id
	LIMIT $2
`

const queryInsertTag = `
	INSERT INTO tag (name)
	VALUES ($1)
	ON CONFLICT (name) DO UPDATE
	SET name = EXCLUDED.name
	RETURNING tag_id
`

const queryAddTagToPin = `
	INSERT INTO pin_tag_relation (pin_id, tag_id)
	SELECT $1, tag_id
	FROM tag
	WHERE tag_id = $2
	  AND EXISTS (
		  SELECT 1 FROM pin
		  WHERE pin_id = $1
		    AND deleted_at IS NULL
	  )
	ON CONFLICT (tag_id, pin_id) DO NOTHING
	RETURNING tag_relation_id
`

const queryRemoveTagFromPin = `
	DELETE FROM pin_tag_relation AS ptr
	USING pin AS p
	WHERE ptr.pin_id = p.pin_id
	  AND ptr.pin_id = $1
	  AND ptr.tag_id = $2
	  AND p.deleted_at IS NULL
	RETURNING ptr.tag_relation_id
`

const queryAddLike = `
	INSERT INTO pin_like (liker_id, pin_id)
	SELECT $1, pin_id
	FROM pin
	WHERE pin_id = $2
	  AND deleted_at IS NULL
	ON CONFLICT (liker_id, pin_id) DO NOTHING
	RETURNING pin_like_id
`

const queryRemoveLike = `
	DELETE FROM pin_like
	WHERE liker_id = $1 AND pin_id = $2
`

const queryGetLikeStats = `
	SELECT COUNT(*), COALESCE(BOOL_OR(liker_id = $1), false)
	FROM pin_like
	WHERE pin_id = $2
`

func (r *Repository) SearchTags(
	ctx context.Context,
	prefix string,
	limit int,
) ([]domain.Tag, error) {

	rows, err := r.pool.Query(ctx, querySearchTags, prefix, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	tags := make([]domain.Tag, 0)
	for rows.Next() {
		var tag domain.Tag
		if err := rows.Scan(&tag.ID, &tag.Name); err != nil {
			return nil, err
		}
		tags = append(tags, tag)
	}

	if err := rows.Err(); err != nil {
		return nil, err
	}
	return tags, nil
}

func (r *Repository) AddTag(
	ctx context.Context,
	pinID int,
	tagID int,
) error {
	var relationID int

	err := r.pool.QueryRow(ctx, queryAddTagToPin, pinID, tagID).Scan(&relationID)
	if err == nil {
		return nil
	}
	if !errors.Is(err, pgx.ErrNoRows) {
		return err
	}

	var pinExists bool
	if err := r.pool.QueryRow(ctx, queryPinExists, pinID).Scan(&pinExists); err != nil {
		return err
	}
	if !pinExists {
		return domain.ErrPinNotFound
	}

	var tagExists bool
	if err := r.pool.QueryRow(ctx, queryTagExists, tagID).Scan(&tagExists); err != nil {
		return err
	}
	if !tagExists {
		return domain.ErrTagNotFound
	}

	return nil
}

func (r *Repository) RemoveTag(
	ctx context.Context,
	pinID int,
	tagID int,
) error {
	var relationID int

	err := r.pool.QueryRow(
		ctx,
		queryRemoveTagFromPin,
		pinID,
		tagID,
	).Scan(&relationID)

	if errors.Is(err, pgx.ErrNoRows) {
		return domain.ErrTagNotFound
	}
	return err
}

func (r *Repository) AddLike(
	ctx context.Context,
	userID int,
	pinID int,
) error {
	var likeID int

	err := r.pool.QueryRow(ctx, queryAddLike, userID, pinID).Scan(&likeID)
	if err == nil {
		return nil
	}
	if !errors.Is(err, pgx.ErrNoRows) {
		return err
	}

	var exists bool
	if err := r.pool.QueryRow(ctx, queryPinExists, pinID).Scan(&exists); err != nil {
		return err
	}
	if !exists {
		return domain.ErrPinNotFound
	}

	return nil
}

func (r *Repository) RemoveLike(ctx context.Context, userID, pinID int) error {
	if userID < 1 || pinID < 1 {
		return domain.ErrValidation
	}
	var exists bool
	if err := r.pool.QueryRow(ctx, queryPinExists, pinID).Scan(&exists); err != nil {
		return err
	}
	if !exists {
		return domain.ErrPinNotFound
	}
	_, err := r.pool.Exec(ctx, queryRemoveLike, userID, pinID)
	return err
}

func (r *Repository) GetLikeStats(ctx context.Context, userID, pinID int) (domain.LikeStats, error) {
	if pinID < 1 {
		return domain.LikeStats{}, domain.ErrValidation
	}
	var exists bool
	if err := r.pool.QueryRow(ctx, queryPinExists, pinID).Scan(&exists); err != nil {
		return domain.LikeStats{}, err
	}
	if !exists {
		return domain.LikeStats{}, domain.ErrPinNotFound
	}
	var result domain.LikeStats
	if err := r.pool.QueryRow(ctx, queryGetLikeStats, userID, pinID).Scan(&result.Count, &result.LikedByMe); err != nil {
		return domain.LikeStats{}, err
	}
	return result, nil
}
