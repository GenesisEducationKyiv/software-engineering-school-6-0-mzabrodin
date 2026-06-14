package scannerclient

import (
	"context"
	"fmt"
	"log/slog"

	"connectrpc.com/connect"

	"github-release-notifier/internal/infrastructure/logging"
	"github-release-notifier/internal/scanner/grpc/gen/scannerv1"
	"github-release-notifier/internal/scanner/grpc/gen/scannerv1/scannerv1connect"
	"github-release-notifier/internal/shared/entity"
)

type Client struct {
	rpc    scannerv1connect.ScannerServiceClient
	closer func()
	log    *slog.Logger
}

func New(httpClient connect.HTTPClient, baseURL string, closer func(), log *slog.Logger) *Client {
	rpc := scannerv1connect.NewScannerServiceClient(
		httpClient,
		baseURL,
		connect.WithGRPC(),
		connect.WithInterceptors(logging.NewConnectCorrelationInterceptor()),
	)

	return newClient(rpc, closer, log)
}

func newClient(rpc scannerv1connect.ScannerServiceClient, closer func(), log *slog.Logger) *Client {
	return &Client{rpc: rpc, closer: closer, log: log.With("component", "scanner-client")}
}

func (c *Client) Scan(ctx context.Context, repos []string) ([]entity.ObservedRelease, error) {
	resp, err := c.rpc.Scan(ctx, connect.NewRequest(&scannerv1.ScanRequest{
		Repositories: repos,
	}))
	if err != nil {
		return nil, fmt.Errorf("scan repositories: %w", err)
	}

	return toObservedReleases(resp.Msg.GetReleases()), nil
}

func (c *Client) Close() error {
	if c.closer != nil {
		c.closer()
	}

	return nil
}
