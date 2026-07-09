package notifier

import (
	"context"
	"log/slog"
	"time"

	"github-release-notifier/internal/infrastructure/config"
)

type maintainer interface {
	Releases(ctx context.Context) error
	Confirmations(ctx context.Context) error
}

type processedPruner interface {
	DeleteProcessedBefore(ctx context.Context, cutoff time.Time) (int64, error)
}

func startMaintenance(
	ctx context.Context,
	retrier maintainer,
	processed processedPruner,
	cfg *config.NotifierConfig,
	log *slog.Logger,
) (<-chan struct{}, context.CancelFunc) {
	maintenanceCtx, cancel := context.WithCancel(ctx)
	done := make(chan struct{})

	go func() {
		defer close(done)
		runMaintenance(maintenanceCtx, retrier, processed, cfg, log)
	}()

	return done, cancel
}

func runMaintenance(
	ctx context.Context,
	retrier maintainer,
	processed processedPruner,
	cfg *config.NotifierConfig,
	log *slog.Logger,
) {
	ticker := time.NewTicker(cfg.RetryInterval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			maintenanceTick(ctx, retrier, processed, cfg, log)
		}
	}
}

func maintenanceTick(
	ctx context.Context,
	retrier maintainer,
	processed processedPruner,
	cfg *config.NotifierConfig,
	log *slog.Logger,
) {
	if err := retrier.Releases(ctx); err != nil {
		log.ErrorContext(ctx, "release retry failed", "error", err)
	}

	if err := retrier.Confirmations(ctx); err != nil {
		log.ErrorContext(ctx, "confirmation retry failed", "error", err)
	}

	removed, err := processed.DeleteProcessedBefore(ctx, time.Now().Add(-cfg.ProcessedTTL))
	if err != nil {
		log.ErrorContext(ctx, "prune processed_releases failed", "error", err)

		return
	}

	if removed > 0 {
		log.InfoContext(ctx, "pruned processed_releases", "removed", removed)
	}
}
