package middleware

import (
	"context"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
)

type PostgresTokenStore struct{ pool *pgxpool.Pool }

func NewPostgresTokenStore(pool *pgxpool.Pool) *PostgresTokenStore {
	return &PostgresTokenStore{pool: pool}
}

func (s *PostgresTokenStore) Revoke(ctx context.Context, tokenID string, expiresAt time.Time) error {
	_, err := s.pool.Exec(ctx, `INSERT INTO revoked_jwt_token (token_id, expires_at) VALUES ($1, $2) ON CONFLICT (token_id) DO NOTHING`, tokenID, expiresAt)
	return err
}

func (s *PostgresTokenStore) IsRevoked(ctx context.Context, tokenID string) (bool, error) {
	var revoked bool
	err := s.pool.QueryRow(ctx, `SELECT EXISTS (SELECT 1 FROM revoked_jwt_token WHERE token_id = $1 AND expires_at > now())`, tokenID).Scan(&revoked)
	if err != nil {
		return false, err
	}
	return revoked, nil
}
