package repository

import (
	"context"
	"fmt"
	"time"

	"github.com/stan-ley-tech/medqueue/internal/apperr"
	"github.com/stan-ley-tech/medqueue/internal/db"
)

type RefreshTokenPG struct{ pgStore }

func NewRefreshTokenRepository(pool *db.Pool) *RefreshTokenPG {
	return &RefreshTokenPG{pgStore{pool}}
}

func (r *RefreshTokenPG) Create(ctx context.Context, userID, tokenHash string, expiresAt time.Time) error {
	_, err := r.pool.Querier(ctx).Exec(ctx, `
		INSERT INTO refresh_tokens (user_id, token_hash, expires_at)
		VALUES ($1, $2, $3)`,
		userID, tokenHash, expiresAt,
	)
	if err != nil {
		return fmt.Errorf("refresh_token_pg: create: %w", err)
	}
	return nil
}

func (r *RefreshTokenPG) GetActiveByHash(ctx context.Context, tokenHash string) (string, error) {
	var userID string
	err := r.pool.Querier(ctx).QueryRow(ctx, `
		SELECT user_id FROM refresh_tokens
		WHERE token_hash = $1 AND revoked_at IS NULL AND expires_at > now()`,
		tokenHash,
	).Scan(&userID)
	if err != nil {
		if isNoRows(err) {
			return "", apperr.Unauthorized("refresh token is invalid or expired")
		}
		return "", fmt.Errorf("refresh_token_pg: get active: %w", err)
	}
	return userID, nil
}

func (r *RefreshTokenPG) Revoke(ctx context.Context, tokenHash string) error {
	_, err := r.pool.Querier(ctx).Exec(ctx,
		`UPDATE refresh_tokens SET revoked_at = now() WHERE token_hash = $1 AND revoked_at IS NULL`, tokenHash)
	if err != nil {
		return fmt.Errorf("refresh_token_pg: revoke: %w", err)
	}
	return nil
}

func (r *RefreshTokenPG) RevokeAllForUser(ctx context.Context, userID string) error {
	_, err := r.pool.Querier(ctx).Exec(ctx,
		`UPDATE refresh_tokens SET revoked_at = now() WHERE user_id = $1 AND revoked_at IS NULL`, userID)
	if err != nil {
		return fmt.Errorf("refresh_token_pg: revoke all: %w", err)
	}
	return nil
}
