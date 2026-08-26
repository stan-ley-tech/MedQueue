package repository

import (
	"context"
	"fmt"
	"time"

	"github.com/stan-ley-tech/medqueue/internal/db"
)

type IdempotencyPG struct{ pgStore }

func NewIdempotencyRepository(pool *db.Pool) *IdempotencyPG {
	return &IdempotencyPG{pgStore{pool}}
}

// Reserve implements a claim-check pattern with a single INSERT ...
// ON CONFLICT statement: if the key is new, it inserts a placeholder row
// and returns found=false so the caller proceeds to do the real work. If
// the key already exists, ON CONFLICT DO NOTHING skips the insert, so the
// caller reads the existing row instead. Concurrent requests bearing the
// same key are serialized by the primary key constraint, not by
// application logic.
func (r *IdempotencyPG) Reserve(ctx context.Context, key, path, requestHash string, ttl time.Duration) (bool, int, []byte, error) {
	cmd, err := r.pool.Querier(ctx).Exec(ctx, `
		INSERT INTO idempotency_keys (key, request_path, request_hash, expires_at)
		VALUES ($1, $2, $3, now() + $4::interval)
		ON CONFLICT (key) DO NOTHING`,
		key, path, requestHash, fmt.Sprintf("%d seconds", int(ttl.Seconds())),
	)
	if err != nil {
		return false, 0, nil, fmt.Errorf("idempotency_pg: reserve: %w", err)
	}
	if cmd.RowsAffected() == 1 {
		return false, 0, nil, nil
	}

	var statusCode *int
	var responseBody []byte
	err = r.pool.Querier(ctx).QueryRow(ctx,
		`SELECT status_code, response_body FROM idempotency_keys WHERE key = $1`, key,
	).Scan(&statusCode, &responseBody)
	if err != nil {
		return false, 0, nil, fmt.Errorf("idempotency_pg: read existing: %w", err)
	}
	if statusCode == nil {
		// Another request is still in flight for this key; the caller
		// should treat this as "in progress" rather than replay a result.
		return true, 0, nil, nil
	}
	return true, *statusCode, responseBody, nil
}

func (r *IdempotencyPG) Complete(ctx context.Context, key string, statusCode int, responseBody []byte) error {
	_, err := r.pool.Querier(ctx).Exec(ctx, `
		UPDATE idempotency_keys SET status_code = $1, response_body = $2 WHERE key = $3`,
		statusCode, responseBody, key,
	)
	if err != nil {
		return fmt.Errorf("idempotency_pg: complete: %w", err)
	}
	return nil
}
