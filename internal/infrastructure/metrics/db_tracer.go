package metrics

import (
	"context"
	"errors"
	"time"

	"github.com/jackc/pgx/v5"
)

type dbOpContextKey struct{}

type dbOp struct {
	operation string
	table     string
}

func WithDBOp(ctx context.Context, operation, table string) context.Context {
	return context.WithValue(ctx, dbOpContextKey{}, dbOp{operation: operation, table: table})
}

type dbStartContextKey struct{}

type PgxTracer struct{}

func NewPgxTracer() *PgxTracer {
	return &PgxTracer{}
}

func (*PgxTracer) TraceQueryStart(
	ctx context.Context,
	_ *pgx.Conn,
	_ pgx.TraceQueryStartData,
) context.Context {
	return context.WithValue(ctx, dbStartContextKey{}, time.Now())
}

func (*PgxTracer) TraceQueryEnd(ctx context.Context, _ *pgx.Conn, data pgx.TraceQueryEndData) {
	operation, table := "unknown", "unknown"
	if op, ok := ctx.Value(dbOpContextKey{}).(dbOp); ok {
		operation, table = op.operation, op.table
	}

	DBQueriesTotal.WithLabelValues(operation, table).Inc()

	if start, ok := ctx.Value(dbStartContextKey{}).(time.Time); ok {
		DBQueryDuration.WithLabelValues(operation, table).Observe(time.Since(start).Seconds())
	}

	if data.Err != nil && !errors.Is(data.Err, pgx.ErrNoRows) {
		DBQueryErrorsTotal.WithLabelValues(operation, table).Inc()
	}
}
