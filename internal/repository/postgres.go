package repository

import "github.com/stan-ley-tech/medqueue/internal/db"

// pgStore is embedded by every Postgres repository so they all resolve
// the active transaction (if any) the same way via pool.Querier(ctx).
type pgStore struct {
	pool *db.Pool
}

// rowScanner is satisfied by pgx.Row, letting scan helpers accept either
// QueryRow's result or a manually iterated Rows without importing pgx.
type rowScanner interface {
	Scan(dest ...any) error
}
