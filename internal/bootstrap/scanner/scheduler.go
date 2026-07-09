package scanner

import (
	"context"
	"log/slog"

	"github-release-notifier/internal/infrastructure/config"
	"github-release-notifier/internal/infrastructure/scheduler"
	"github-release-notifier/internal/scanner/usecase/watch"
)

func startScheduler(
	ctx context.Context,
	watchUC *watch.UseCase,
	cfg *config.ScannerConfig,
	log *slog.Logger,
) (<-chan struct{}, context.CancelFunc) {
	schedulerCtx, cancel := context.WithCancel(ctx)
	done := make(chan struct{})

	sched := scheduler.New(watchUC, cfg.ScanInterval, log)

	go func() {
		defer close(done)
		sched.Start(schedulerCtx)
	}()

	return done, cancel
}
