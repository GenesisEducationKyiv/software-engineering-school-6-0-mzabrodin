package mailer

import (
	"context"
	"fmt"
	"log/slog"

	"github.com/wneessen/go-mail"
)

type smtpSender struct {
	client *mail.Client
	log    *slog.Logger
}

func (s *smtpSender) sendBatch(ctx context.Context, messages []*mail.Msg) []error {
	errs := make([]error, len(messages))

	if err := s.client.DialWithContext(ctx); err != nil {
		dialErr := fmt.Errorf("dial SMTP: %w", err)
		for i := range errs {
			errs[i] = dialErr
		}

		return errs
	}

	defer func() {
		if err := s.client.Close(); err != nil {
			s.log.ErrorContext(ctx, "failed to close SMTP connection", "error", err)
		}
	}()

	for i, msg := range messages {
		if err := s.client.Send(msg); err != nil {
			errs[i] = fmt.Errorf("send email: %w", err)
		}
	}

	return errs
}
