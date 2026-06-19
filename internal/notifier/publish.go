package notifier

import (
	"context"
	"log/slog"
)

func TryPublish(ctx context.Context, log *slog.Logger, event string, publish func() error) {
	if err := publish(); err != nil {
		log.WarnContext(ctx, "failed to publish notification event", "event", event, "error", err)
	}
}
