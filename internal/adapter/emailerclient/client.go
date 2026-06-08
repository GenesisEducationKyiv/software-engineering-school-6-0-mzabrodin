package emailerclient

import (
	"context"
	"fmt"
	"io"
	"log/slog"
	"sync"
	"time"

	"google.golang.org/grpc"

	emailerv1 "github-release-notifier/internal/adapter/grpc/gen/emailer/v1"
	"github-release-notifier/internal/entity"
	"github-release-notifier/internal/infrastructure/logging"
)

const confirmationTimeout = 30 * time.Second

type emailerService interface {
	SendConfirmation(
		ctx context.Context,
		in *emailerv1.SendConfirmationRequest,
		opts ...grpc.CallOption,
	) (*emailerv1.SendConfirmationResponse, error)
	SendReleaseNotifications(
		ctx context.Context,
		in *emailerv1.SendReleaseNotificationsRequest,
		opts ...grpc.CallOption,
	) (*emailerv1.SendReleaseNotificationsResponse, error)
}

type Client struct {
	rpc      emailerService
	conn     io.Closer
	log      *slog.Logger
	inflight sync.WaitGroup
}

func New(conn *grpc.ClientConn, log *slog.Logger) *Client {
	return newClient(emailerv1.NewEmailerServiceClient(conn), conn, log)
}

func newClient(rpc emailerService, conn io.Closer, log *slog.Logger) *Client {
	return &Client{rpc: rpc, conn: conn, log: log.With("component", "emailer-client")}
}

func (c *Client) SendConfirmation(ctx context.Context, to, repo, confirmURL string) {
	c.inflight.Add(1)

	go func() {
		defer c.inflight.Done()

		sendCtx, cancel := context.WithTimeout(context.WithoutCancel(ctx), confirmationTimeout)
		defer cancel()

		sendCtx = logging.WithOutgoingIDs(sendCtx)

		_, err := c.rpc.SendConfirmation(sendCtx, &emailerv1.SendConfirmationRequest{
			To:         to,
			Repo:       repo,
			ConfirmUrl: confirmURL,
		})
		if err != nil {
			c.log.ErrorContext(sendCtx, "failed to send confirmation email", "to", to, "repo", repo, "error", err)
		}
	}()
}

func (c *Client) SendReleaseNotifications(
	ctx context.Context,
	notifications []entity.ReleaseNotification,
) entity.BatchResult {
	ctx = logging.WithOutgoingIDs(ctx)

	resp, err := c.rpc.SendReleaseNotifications(ctx, &emailerv1.SendReleaseNotificationsRequest{
		Notifications: toProtoNotifications(notifications),
	})
	if err != nil {
		c.log.ErrorContext(ctx, "failed to send release notifications", "count", len(notifications), "error", err)

		return entity.BatchResult{Failed: recipients(notifications)}
	}

	return entity.BatchResult{Sent: int(resp.GetSent()), Failed: resp.GetFailed()}
}

func (c *Client) Close() error {
	c.inflight.Wait()

	if c.conn != nil {
		if err := c.conn.Close(); err != nil {
			return fmt.Errorf("close emailer connection: %w", err)
		}
	}

	return nil
}
