package compensationclient

import (
	"context"
	"crypto/tls"
	"fmt"
	"net"
	"net/http"

	"connectrpc.com/connect"
	"golang.org/x/net/http2"

	"github-release-notifier/internal/shared/grpc/gen/compensationv1"
	"github-release-notifier/internal/shared/grpc/gen/compensationv1/compensationv1connect"
)

type Client struct {
	httpClient *http.Client
	client     compensationv1connect.CompensationServiceClient
}

func Dial(addr string) (*Client, error) {
	httpClient := &http.Client{
		Transport: &http2.Transport{
			AllowHTTP: true,
			DialTLSContext: func(ctx context.Context, network, addr string, _ *tls.Config) (net.Conn, error) {
				var d net.Dialer

				return d.DialContext(ctx, network, addr)
			},
		},
	}

	client := compensationv1connect.NewCompensationServiceClient(
		httpClient, "http://"+addr, connect.WithGRPC())

	return &Client{httpClient: httpClient, client: client}, nil
}

func (c *Client) Compensate(ctx context.Context, sagaID, sagaType, email, repoName string) (bool, error) {
	resp, err := c.client.Compensate(ctx, connect.NewRequest(&compensationv1.CompensateRequest{
		SagaId:   sagaID,
		SagaType: sagaType,
		Email:    email,
		RepoName: repoName,
	}))
	if err != nil {
		return false, fmt.Errorf("compensate rpc: %w", err)
	}

	return resp.Msg.GetRolledBack(), nil
}

func (c *Client) Close() error {
	c.httpClient.CloseIdleConnections()

	return nil
}
