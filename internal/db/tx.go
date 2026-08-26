package db

import (
	"context"
	"errors"
	"fmt"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
)

// DBTX is satisfied by both *pgxpool.Pool and pgx.Tx, so repositories can
// be written against it and transparently work whether or not they are
// running inside a transaction.
type DBTX interface {
	Exec(ctx context.Context, sql string, args ...any) (pgconn.CommandTag, error)
	Query(ctx context.Context, sql string, args ...any) (pgx.Rows, error)
	QueryRow(ctx context.Context, sql string, args ...any) pgx.Row
}

type txKey struct{}

// WithTx runs fn inside a database transaction. If fn returns an error the
// transaction is rolled back; otherwise it is committed. Nested calls
// reuse the outer transaction rather than opening a new one, so services
// can compose repository calls that each independently call WithTx.
func (p *Pool) WithTx(ctx context.Context, fn func(ctx context.Context) error) error {
	if tx, ok := ctx.Value(txKey{}).(pgx.Tx); ok && tx != nil {
		return fn(ctx)
	}

	tx, err := p.Begin(ctx)
	if err != nil {
		return fmt.Errorf("db: begin tx: %w", err)
	}

	ctx = context.WithValue(ctx, txKey{}, tx)

	if err := fn(ctx); err != nil {
		if rbErr := tx.Rollback(ctx); rbErr != nil && !errors.Is(rbErr, pgx.ErrTxClosed) {
			return fmt.Errorf("db: rollback after error %w: %v", err, rbErr)
		}
		return err
	}

	if err := tx.Commit(ctx); err != nil {
		return fmt.Errorf("db: commit tx: %w", err)
	}
	return nil
}

// Querier resolves the active DBTX for ctx: the transaction started by
// WithTx if one is in progress, otherwise the pool itself.
func (p *Pool) Querier(ctx context.Context) DBTX {
	if tx, ok := ctx.Value(txKey{}).(pgx.Tx); ok && tx != nil {
		return tx
	}
	return p.Pool
}
