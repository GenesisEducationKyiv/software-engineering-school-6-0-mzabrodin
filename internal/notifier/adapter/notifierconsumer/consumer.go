package notifierconsumer

import (
	"context"
	"fmt"
	"log/slog"

	"buf.build/go/protovalidate"
	"google.golang.org/protobuf/proto"

	"github-release-notifier/internal/infrastructure/broker"
	"github-release-notifier/internal/notifier"
	"github-release-notifier/internal/notifier/grpc/gen/notifierv1"
)

type mailer interface {
	SendConfirmation(ctx context.Context, to, repo, confirmURL string)
	SendReleaseNotifications(ctx context.Context, notifications []notifier.ReleaseNotification) notifier.BatchResult
}

type Consumer struct {
	mailer    mailer
	validator protovalidate.Validator
	log       *slog.Logger
}

func New(m mailer, validator protovalidate.Validator, log *slog.Logger) *Consumer {
	return &Consumer{mailer: m, validator: validator, log: log.With("component", "notifier-consumer")}
}

func (c *Consumer) HandleConfirmation(ctx context.Context, data []byte) error {
	var msg notifierv1.SendConfirmationRequest
	if err := c.unmarshalAndValidate(data, &msg); err != nil {
		return err
	}

	c.mailer.SendConfirmation(ctx, msg.GetTo(), msg.GetRepo(), msg.GetConfirmUrl())

	return nil
}

func (c *Consumer) HandleRelease(ctx context.Context, data []byte) error {
	var msg notifierv1.SendReleaseNotificationsRequest
	if err := c.unmarshalAndValidate(data, &msg); err != nil {
		return err
	}

	result := c.mailer.SendReleaseNotifications(ctx, toEntityNotifications(msg.GetNotifications()))

	if result.Sent == 0 && len(result.Failed) > 0 {
		return fmt.Errorf("all %d release notifications failed", len(result.Failed))
	}

	if len(result.Failed) > 0 {
		c.log.WarnContext(ctx, "some release notifications failed", "failed", len(result.Failed), "sent", result.Sent)
	}

	return nil
}

func (c *Consumer) unmarshalAndValidate(data []byte, msg proto.Message) error {
	if err := proto.Unmarshal(data, msg); err != nil {
		return fmt.Errorf("%w: unmarshal: %w", broker.ErrTerminal, err)
	}

	if err := c.validator.Validate(msg); err != nil {
		return fmt.Errorf("%w: validate: %w", broker.ErrTerminal, err)
	}

	return nil
}
