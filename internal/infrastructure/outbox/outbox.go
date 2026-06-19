package outbox

import (
	"context"
	"embed"
	"fmt"

	"github.com/jackc/pgx/v5"
)

//go:embed migrations/*.sql
var Migrations embed.FS

func Enqueue(ctx context.Context, tx pgx.Tx, subject string, payload []byte) error {
	if _, err := tx.Exec(ctx,
		`INSERT INTO outbox_messages (subject, payload) VALUES ($1, $2)`,
		subject, payload,
	); err != nil {
		return fmt.Errorf("enqueue outbox message to %q: %w", subject, err)
	}

	return nil
}
