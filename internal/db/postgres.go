// Package db wraps the PostgreSQL connection pool and the transaction
// helper used throughout the repository layer.
package db

import (
	"context"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
)

type Pool struct {
	*pgxpool.Pool
}

type Options struct {
	URL             string
	MaxOpenConns    int32
	MaxIdleConns    int32
	ConnMaxLifetime time.Duration
}

// Connect builds a pooled PostgreSQL connection and verifies it with a
// ping so startup fails fast on bad configuration instead of surfacing
// the failure on the first request.
func Connect(ctx context.Context, opts Options) (*Pool, error) {
	cfg, err := pgxpool.ParseConfig(opts.URL)
	if err != nil {
		return nil, fmt.Errorf("db: parse config: %w", err)
	}

	if opts.MaxOpenConns > 0 {
		cfg.MaxConns = opts.MaxOpenConns
	}
	if opts.MaxIdleConns > 0 {
		cfg.MinConns = opts.MaxIdleConns
	}
	if opts.ConnMaxLifetime > 0 {
		cfg.MaxConnLifetime = opts.ConnMaxLifetime
	}

	pool, err := pgxpool.NewWithConfig(ctx, cfg)
	if err != nil {
		return nil, fmt.Errorf("db: create pool: %w", err)
	}

	pingCtx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()
	if err := pool.Ping(pingCtx); err != nil {
		pool.Close()
		return nil, fmt.Errorf("db: ping: %w", err)
	}

	return &Pool{pool}, nil
}
