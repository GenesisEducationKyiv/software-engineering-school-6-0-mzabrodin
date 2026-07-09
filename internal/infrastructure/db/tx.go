package db

import (
	"context"
	"fmt"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"
)

type Querier interface {
	Exec(ctx context.Context, sql string, args ...any) (pgconn.CommandTag, error)
	Query(ctx context.Context, sql string, args ...any) (pgx.Rows, error)
	QueryRow(ctx context.Context, sql string, args ...any) pgx.Row
}

type txContextKey struct{}

type Transactor struct {
	pool *pgxpool.Pool
}

func NewTransactor(pool *pgxpool.Pool) *Transactor {
	return &Transactor{pool: pool}
}

func (t *Transactor) Within(ctx context.Context, fn func(ctx context.Context) error) error {
	if _, ok := txFromContext(ctx); ok {
		return fn(ctx)
	}

	tx, err := t.pool.Begin(ctx)
	if err != nil {
		return fmt.Errorf("begin tx: %w", err)
	}
	defer func() { _ = tx.Rollback(ctx) }()

	if err := fn(context.WithValue(ctx, txContextKey{}, tx)); err != nil {
		return err
	}

	if err := tx.Commit(ctx); err != nil {
		return fmt.Errorf("commit tx: %w", err)
	}

	return nil
}

func FromContext(ctx context.Context, pool *pgxpool.Pool) Querier {
	if tx, ok := txFromContext(ctx); ok {
		return tx
	}

	return pool
}

func Tx(ctx context.Context) (pgx.Tx, bool) {
	return txFromContext(ctx)
}

func txFromContext(ctx context.Context) (pgx.Tx, bool) {
	tx, ok := ctx.Value(txContextKey{}).(pgx.Tx)

	return tx, ok
}
