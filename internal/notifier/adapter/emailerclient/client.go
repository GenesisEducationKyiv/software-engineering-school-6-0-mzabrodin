package emailerclient

import (
	"context"
	"log/slog"
	"sync"
	"time"

	"connectrpc.com/connect"

	"github-release-notifier/internal/infrastructure/logging"
	"github-release-notifier/internal/notifier"
	"github-release-notifier/internal/notifier/grpc/gen/emailerv1"
	"github-release-notifier/internal/notifier/grpc/gen/emailerv1/emailerv1connect"
)

const confirmationTimeout = 30 * time.Second

type Client struct {
	rpc      emailerv1connect.EmailerServiceClient
	closer   func()
	log      *slog.Logger
	inflight sync.WaitGroup
}

func New(httpClient connect.HTTPClient, baseURL string, closer func(), log *slog.Logger) *Client {
	rpc := emailerv1connect.NewEmailerServiceClient(
		httpClient,
		baseURL,
		connect.WithGRPC(),
		connect.WithInterceptors(logging.NewConnectCorrelationInterceptor()),
	)

	return newClient(rpc, closer, log)
}

func newClient(rpc emailerv1connect.EmailerServiceClient, closer func(), log *slog.Logger) *Client {
	return &Client{rpc: rpc, closer: closer, log: log.With("component", "emailer-client")}
}

func (c *Client) SendConfirmation(ctx context.Context, to, repo, confirmURL string) {
	c.inflight.Add(1)

	go func() {
		defer c.inflight.Done()

		sendCtx, cancel := context.WithTimeout(context.WithoutCancel(ctx), confirmationTimeout)
		defer cancel()

		_, err := c.rpc.SendConfirmation(sendCtx, connect.NewRequest(&emailerv1.SendConfirmationRequest{
			To:         to,
			Repo:       repo,
			ConfirmUrl: confirmURL,
		}))
		if err != nil {
			c.log.ErrorContext(sendCtx, "failed to send confirmation email", "to", to, "repo", repo, "error", err)
		}
	}()
}

func (c *Client) SendReleaseNotifications(
	ctx context.Context,
	notifications []notifier.ReleaseNotification,
) notifier.BatchResult {
	resp, err := c.rpc.SendReleaseNotifications(ctx, connect.NewRequest(&emailerv1.SendReleaseNotificationsRequest{
		Notifications: toProtoNotifications(notifications),
	}))
	if err != nil {
		c.log.ErrorContext(ctx, "failed to send release notifications", "count", len(notifications), "error", err)

		return notifier.BatchResult{Failed: recipients(notifications)}
	}

	return notifier.BatchResult{Sent: int(resp.Msg.GetSent()), Failed: resp.Msg.GetFailed()}
}

func (c *Client) Close() error {
	c.inflight.Wait()

	if c.closer != nil {
		c.closer()
	}

	return nil
}
