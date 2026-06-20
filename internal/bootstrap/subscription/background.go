package subscription

import (
	"context"
	"log/slog"
	"sync"
	"time"

	"github-release-notifier/internal/infrastructure/outbox"
	"github-release-notifier/internal/subscription/usecase/cleanup"
)

func startBackground(
	ctx context.Context,
	relay *outbox.Relay,
	cleanupUC *cleanup.UseCase,
	cleanupInterval time.Duration,
	log *slog.Logger,
) (<-chan struct{}, context.CancelFunc) {
	bgCtx, cancel := context.WithCancel(ctx)
	done := make(chan struct{})

	var wg sync.WaitGroup
	wg.Add(2)

	go func() {
		defer wg.Done()
		relay.Run(bgCtx)
	}()

	go func() {
		defer wg.Done()
		runCleanup(bgCtx, cleanupUC, cleanupInterval, log)
	}()

	go func() {
		wg.Wait()
		close(done)
	}()

	return done, cancel
}

func runCleanup(ctx context.Context, uc *cleanup.UseCase, interval time.Duration, log *slog.Logger) {
	tick := func() {
		if err := uc.Run(ctx); err != nil {
			log.ErrorContext(ctx, "pending cleanup failed", "error", err)
		}
	}

	tick()

	ticker := time.NewTicker(interval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			tick()
		}
	}
}
