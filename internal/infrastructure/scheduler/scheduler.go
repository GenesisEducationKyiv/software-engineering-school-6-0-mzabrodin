package scheduler

import (
	"context"
	"log/slog"
	"time"

	"github.com/google/uuid"

	"github-release-notifier/internal/infrastructure/logging"
)

type runner interface {
	Run(ctx context.Context) error
}

type Scheduler struct {
	runner   runner
	interval time.Duration
	log      *slog.Logger
}

func New(r runner, interval time.Duration, log *slog.Logger) *Scheduler {
	return &Scheduler{
		runner:   r,
		interval: interval,
		log:      log.With("component", "scheduler"),
	}
}

func (s *Scheduler) Start(ctx context.Context) {
	s.log.Info("scheduler started", "interval", s.interval)

	s.run(ctx)

	ticker := time.NewTicker(s.interval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			s.log.Info("scheduler stopped")
			return
		case <-ticker.C:
			s.run(ctx)
		}
	}
}

func (s *Scheduler) run(ctx context.Context) {
	ctx = logging.WithScanID(ctx, uuid.NewString())
	if err := s.runner.Run(ctx); err != nil {
		s.log.ErrorContext(ctx, "scan pass failed", "error", err)
	}
}
