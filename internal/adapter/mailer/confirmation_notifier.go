package mailer

import (
	"context"
	"log/slog"
	"sync"
	"time"
)

const confirmationSendTimeout = time.Minute

type confirmationSender interface {
	SendConfirmation(ctx context.Context, to, repo, confirmURL string) error
}

type ConfirmationNotifier struct {
	sender confirmationSender
	wg     sync.WaitGroup
	log    *slog.Logger
}

func NewConfirmationNotifier(sender confirmationSender, log *slog.Logger) *ConfirmationNotifier {
	return &ConfirmationNotifier{sender: sender, log: log.With("component", "confirmation_notifier")}
}

func (c *ConfirmationNotifier) SendConfirmation(ctx context.Context, to, repo, confirmURL string) {
	ctx = context.WithoutCancel(ctx)

	c.wg.Add(1)
	go func() {
		defer c.wg.Done()

		ctx, cancel := context.WithTimeout(ctx, confirmationSendTimeout)
		defer cancel()

		if err := c.sender.SendConfirmation(ctx, to, repo, confirmURL); err != nil {
			c.log.ErrorContext(ctx, "failed to send confirmation email", "email", to, "repo", repo, "error", err)
		} else {
			c.log.InfoContext(ctx, "confirmation email sent", "email", to, "repo", repo)
		}
	}()
}

func (c *ConfirmationNotifier) Shutdown() {
	c.wg.Wait()
}
