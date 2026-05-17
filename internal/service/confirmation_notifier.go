package service

import (
	"log/slog"
	"sync"
)

type asyncMailer interface {
	SendConfirmation(to, repo, confirmURL string) error
}

type ConfirmationNotifier struct {
	asyncMailer asyncMailer
	wg          sync.WaitGroup
}

func NewConfirmationNotifier(m asyncMailer) *ConfirmationNotifier {
	return &ConfirmationNotifier{asyncMailer: m}
}

func (c *ConfirmationNotifier) SendConfirmation(to, repo, confirmURL string) error {
	c.wg.Add(1)
	go func() {
		defer c.wg.Done()
		if err := c.asyncMailer.SendConfirmation(to, repo, confirmURL); err != nil {
			slog.Error("failed to send confirmation email", "email", to, "error", err)
		}
	}()

	return nil
}

func (c *ConfirmationNotifier) Shutdown() {
	c.wg.Wait()
}
