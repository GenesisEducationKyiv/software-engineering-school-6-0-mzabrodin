package repository

import (
	"context"
	"errors"
	"fmt"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"github-release-notifier/internal/infrastructure/db"
	"github-release-notifier/internal/infrastructure/metrics"
	"github-release-notifier/internal/saga/domain"
)

type SagaRepository struct {
	pool *pgxpool.Pool
}

func NewSagaRepository(pool *pgxpool.Pool) *SagaRepository {
	return &SagaRepository{pool: pool}
}

func (r *SagaRepository) Start(ctx context.Context, s domain.Saga) error {
	ctx = metrics.WithDBOp(ctx, "start", "sagas")

	if _, err := db.FromContext(ctx, r.pool).Exec(ctx, `
		INSERT INTO sagas (id, type, state, email, repo_name)
		VALUES ($1, $2, $3, $4, $5)
		ON CONFLICT (type, email, repo_name)
		DO UPDATE SET id = EXCLUDED.id, state = EXCLUDED.state, updated_at = now()
	`, s.ID, s.Type, domain.StatePending, s.Email, s.RepoName); err != nil {
		return fmt.Errorf("start saga: %w", err)
	}

	return nil
}

func (r *SagaRepository) TransitionByID(ctx context.Context, id uuid.UUID, to domain.State) (bool, error) {
	ctx = metrics.WithDBOp(ctx, "transition_by_id", "sagas")

	tag, err := db.FromContext(ctx, r.pool).Exec(ctx, `
		UPDATE sagas SET state = $2, updated_at = now()
		WHERE id = $1 AND state = ANY($3)
	`, id, to, sourceStrings(to))
	if err != nil {
		return false, fmt.Errorf("transition saga %s -> %s: %w", id, to, err)
	}

	return tag.RowsAffected() > 0, nil
}

func (r *SagaRepository) TransitionByEmailRepo(
	ctx context.Context,
	t domain.SagaType,
	email, repoName string,
	to domain.State,
) (bool, error) {
	ctx = metrics.WithDBOp(ctx, "transition_by_email_repo", "sagas")

	tag, err := db.FromContext(ctx, r.pool).Exec(ctx, `
		UPDATE sagas SET state = $4, updated_at = now()
		WHERE type = $1 AND email = $2 AND repo_name = $3 AND state = ANY($5)
	`, t, email, repoName, to, sourceStrings(to))
	if err != nil {
		return false, fmt.Errorf("transition saga (%s, %s) -> %s: %w", email, repoName, to, err)
	}

	return tag.RowsAffected() > 0, nil
}

func (r *SagaRepository) Get(ctx context.Context, id uuid.UUID) (domain.Saga, bool, error) {
	ctx = metrics.WithDBOp(ctx, "get", "sagas")

	var s domain.Saga

	err := db.FromContext(ctx, r.pool).QueryRow(ctx, `
		SELECT id, type, state, email, repo_name, created_at, updated_at
		FROM sagas WHERE id = $1
	`, id).Scan(&s.ID, &s.Type, &s.State, &s.Email, &s.RepoName, &s.CreatedAt, &s.UpdatedAt)
	if errors.Is(err, pgx.ErrNoRows) {
		return domain.Saga{}, false, nil
	}

	if err != nil {
		return domain.Saga{}, false, fmt.Errorf("get saga %s: %w", id, err)
	}

	return s, true, nil
}

func sourceStrings(to domain.State) []string {
	sources := domain.SourceStates(to)
	out := make([]string, len(sources))

	for i, s := range sources {
		out[i] = string(s)
	}

	return out
}
