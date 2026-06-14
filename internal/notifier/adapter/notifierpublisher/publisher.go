package notifierpublisher

import (
	"context"
	"log/slog"
	"sync"
	"time"

	"google.golang.org/protobuf/proto"

	"github-release-notifier/internal/notifier"
	"github-release-notifier/internal/notifier/grpc/gen/notifierv1"
)

const confirmationTimeout = 30 * time.Second

type publisher interface {
	Publish(ctx context.Context, subject string, data []byte) error
}

type Publisher struct {
	pub      publisher
	log      *slog.Logger
	inflight sync.WaitGroup
}

func New(pub publisher, log *slog.Logger) *Publisher {
	return &Publisher{pub: pub, log: log.With("component", "notifier-publisher")}
}

func (p *Publisher) SendConfirmation(ctx context.Context, to, repo, confirmURL string) {
	p.inflight.Add(1)

	go func() {
		defer p.inflight.Done()

		sendCtx, cancel := context.WithTimeout(context.WithoutCancel(ctx), confirmationTimeout)
		defer cancel()

		data, err := proto.Marshal(&notifierv1.SendConfirmationRequest{
			To:         to,
			Repo:       repo,
			ConfirmUrl: confirmURL,
		})
		if err != nil {
			p.log.ErrorContext(sendCtx, "failed to marshal confirmation", "to", to, "repo", repo, "error", err)

			return
		}

		if err := p.pub.Publish(sendCtx, notifier.SubjectConfirmation, data); err != nil {
			p.log.ErrorContext(sendCtx, "failed to publish confirmation email", "to", to, "repo", repo, "error", err)
		}
	}()
}

func (p *Publisher) SendReleaseNotifications(
	ctx context.Context,
	notifications []notifier.ReleaseNotification,
) notifier.BatchResult {
	data, err := proto.Marshal(&notifierv1.SendReleaseNotificationsRequest{
		Notifications: toProtoNotifications(notifications),
	})
	if err != nil {
		p.log.ErrorContext(ctx, "failed to marshal release notifications", "count", len(notifications), "error", err)

		return notifier.BatchResult{Failed: recipients(notifications)}
	}

	if err := p.pub.Publish(ctx, notifier.SubjectRelease, data); err != nil {
		p.log.ErrorContext(ctx, "failed to publish release notifications", "count", len(notifications), "error", err)

		return notifier.BatchResult{Failed: recipients(notifications)}
	}

	return notifier.BatchResult{Sent: len(notifications)}
}

func (p *Publisher) Close() error {
	p.inflight.Wait()

	return nil
}
