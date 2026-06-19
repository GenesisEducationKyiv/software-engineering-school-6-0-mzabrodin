package outbox

import (
	"context"
	"fmt"
	"log/slog"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

type Publisher interface {
	Publish(ctx context.Context, subject string, data []byte) error
}

type message struct {
	id      int64
	subject string
	payload []byte
}

type Relay struct {
	pool      *pgxpool.Pool
	publisher Publisher
	interval  time.Duration
	batchSize int
	nudge     chan struct{}
	log       *slog.Logger
}

func NewRelay(
	pool *pgxpool.Pool,
	publisher Publisher,
	interval time.Duration,
	batchSize int,
	log *slog.Logger,
) *Relay {
	return &Relay{
		pool:      pool,
		publisher: publisher,
		interval:  interval,
		batchSize: batchSize,
		nudge:     make(chan struct{}, 1),
		log:       log.With("component", "outbox-relay"),
	}
}

func (r *Relay) Notify() {
	select {
	case r.nudge <- struct{}{}:
	default:
	}
}

func (r *Relay) Run(ctx context.Context) {
	ticker := time.NewTicker(r.interval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			r.tryFlush(ctx)
		case <-r.nudge:
			r.tryFlush(ctx)
		}
	}
}

func (r *Relay) tryFlush(ctx context.Context) {
	if err := r.flush(ctx); err != nil {
		r.log.ErrorContext(ctx, "outbox flush failed", "error", err)
	}
}

func (r *Relay) flush(ctx context.Context) error {
	for {
		published, err := r.flushBatch(ctx)
		if err != nil {
			return err
		}

		if published < r.batchSize {
			return nil
		}
	}
}

func (r *Relay) flushBatch(ctx context.Context) (int, error) {
	tx, err := r.pool.Begin(ctx)
	if err != nil {
		return 0, fmt.Errorf("begin outbox tx: %w", err)
	}
	defer func() { _ = tx.Rollback(ctx) }()

	messages, err := selectLocked(ctx, tx, r.batchSize)
	if err != nil {
		return 0, err
	}

	if len(messages) == 0 {
		return 0, nil
	}

	ids, publishErr := r.publishAll(ctx, messages)
	if len(ids) == 0 {
		return 0, publishErr
	}

	if _, err := tx.Exec(ctx, `DELETE FROM outbox_messages WHERE id = ANY($1)`, ids); err != nil {
		return 0, fmt.Errorf("delete published messages: %w", err)
	}

	if err := tx.Commit(ctx); err != nil {
		return 0, fmt.Errorf("commit outbox tx: %w", err)
	}

	return len(ids), publishErr
}

func (r *Relay) publishAll(ctx context.Context, messages []message) ([]int64, error) {
	ids := make([]int64, 0, len(messages))

	for _, m := range messages {
		if err := r.publisher.Publish(ctx, m.subject, m.payload); err != nil {
			return ids, fmt.Errorf("publish outbox message %d to %q: %w", m.id, m.subject, err)
		}

		ids = append(ids, m.id)
	}

	return ids, nil
}

func selectLocked(ctx context.Context, tx pgx.Tx, batchSize int) ([]message, error) {
	rows, err := tx.Query(ctx,
		`SELECT id, subject, payload
		   FROM outbox_messages
		  ORDER BY id
		  LIMIT $1
		  FOR UPDATE SKIP LOCKED`,
		batchSize,
	)
	if err != nil {
		return nil, fmt.Errorf("select outbox messages: %w", err)
	}
	defer rows.Close()

	var messages []message
	for rows.Next() {
		var m message
		if err := rows.Scan(&m.id, &m.subject, &m.payload); err != nil {
			return nil, fmt.Errorf("scan outbox message: %w", err)
		}

		messages = append(messages, m)
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate outbox messages: %w", err)
	}

	return messages, nil
}
