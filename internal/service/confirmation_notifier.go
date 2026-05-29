package service

import (
	"context"
	"log/slog"
	"sync"
)

type asyncMailer interface {
	SendConfirmation(ctx context.Context, to, repo, confirmURL string) error
}

type ConfirmationNotifier struct {
	asyncMailer asyncMailer
	wg          sync.WaitGroup
	log         *slog.Logger
}

func NewConfirmationNotifier(m asyncMailer, log *slog.Logger) *ConfirmationNotifier {
	return &ConfirmationNotifier{asyncMailer: m, log: log.With("component", "confirmation_notifier")}
}

func (c *ConfirmationNotifier) SendConfirmation(to, repo, confirmURL string) error {
	c.wg.Add(1)
	go func() {
		defer c.wg.Done()
		if err := c.asyncMailer.SendConfirmation(context.Background(), to, repo, confirmURL); err != nil {
			c.log.Error("failed to send confirmation email", "email", to, "error", err)
		}
	}()

	return nil
}

func (c *ConfirmationNotifier) Shutdown() {
	c.wg.Wait()
}
