//go:build integration

package integration

import (
	"context"
	"net"
	"path/filepath"
	"testing"
	"time"

	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
	"github.com/stretchr/testify/suite"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials"

	"github-release-notifier/internal/infrastructure/logging"
	"github-release-notifier/internal/notifier"
	"github-release-notifier/internal/notifier/adapter/emailerclient"
	"github-release-notifier/internal/notifier/adapter/emailerserver"
	"github-release-notifier/internal/notifier/certgen"
	"github-release-notifier/internal/notifier/tlsconfig"
)

type mockEmailerMailer struct{ mock.Mock }

func (m *mockEmailerMailer) SendConfirmation(ctx context.Context, to, repo, confirmURL string) {
	m.Called(ctx, to, repo, confirmURL)
}

func (m *mockEmailerMailer) SendReleaseNotifications(
	ctx context.Context,
	notifications []notifier.ReleaseNotification,
) notifier.BatchResult {
	args := m.Called(ctx, notifications)
	out, _ := args.Get(0).(notifier.BatchResult)
	return out
}

type EmailerGRPCSuite struct {
	suite.Suite

	mailer *mockEmailerMailer
	client *emailerclient.Client
}

func TestEmailerGRPCSuite(t *testing.T) {
	suite.Run(t, new(EmailerGRPCSuite))
}

func (s *EmailerGRPCSuite) SetupTest() {
	dir := s.T().TempDir()
	s.Require().NoError(certgen.Write(dir))

	serverCfg, err := tlsconfig.ServerTLS(
		filepath.Join(dir, "server.crt"),
		filepath.Join(dir, "server.key"),
		filepath.Join(dir, "ca.crt"),
	)
	s.Require().NoError(err)

	clientCfg, err := tlsconfig.ClientTLS(
		filepath.Join(dir, "client.crt"),
		filepath.Join(dir, "client.key"),
		filepath.Join(dir, "ca.crt"),
		"localhost",
	)
	s.Require().NoError(err)

	s.mailer = &mockEmailerMailer{}
	handler := emailerserver.NewServer(s.mailer, testLogger)
	srv, err := emailerserver.NewGRPCServer(handler, serverCfg, testLogger)
	s.Require().NoError(err)

	var lc net.ListenConfig
	lis, err := lc.Listen(s.T().Context(), "tcp", "127.0.0.1:0")
	s.Require().NoError(err)

	go func() {
		_ = srv.Serve(lis)
	}()

	_, port, err := net.SplitHostPort(lis.Addr().String())
	s.Require().NoError(err)

	conn, err := grpc.NewClient(
		"localhost:"+port,
		grpc.WithTransportCredentials(credentials.NewTLS(clientCfg)),
	)
	s.Require().NoError(err)

	s.client = emailerclient.New(conn, testLogger)

	s.T().Cleanup(func() {
		require.NoError(s.T(), s.client.Close())
		srv.Stop()
	})
}

func (s *EmailerGRPCSuite) TearDownTest() {
	s.mailer.AssertExpectations(s.T())
}

func (s *EmailerGRPCSuite) TestSendReleaseNotifications() {
	notifications := []notifier.ReleaseNotification{{
		To:             "a@example.com",
		Repo:           "owner/repo",
		Tag:            "v1.0.0",
		ReleaseURL:     "https://github.com/owner/repo/releases/tag/v1.0.0",
		UnsubscribeURL: "https://example.com/unsubscribe/a",
	}}

	s.mailer.On("SendReleaseNotifications", mock.Anything, notifications).
		Return(notifier.BatchResult{Sent: 1, Failed: []string{"b@example.com"}})

	result := s.client.SendReleaseNotifications(s.T().Context(), notifications)

	s.Equal(1, result.Sent)
	s.Equal([]string{"b@example.com"}, result.Failed)
}

func (s *EmailerGRPCSuite) TestPropagatesCorrelationIDs() {
	notifications := []notifier.ReleaseNotification{{
		To:             "a@example.com",
		Repo:           "owner/repo",
		Tag:            "v1.0.0",
		ReleaseURL:     "https://github.com/owner/repo/releases/tag/v1.0.0",
		UnsubscribeURL: "https://example.com/unsubscribe/a",
	}}

	var gotRequestID, gotScanID string
	s.mailer.On("SendReleaseNotifications", mock.Anything, notifications).
		Run(func(args mock.Arguments) {
			ctx, _ := args.Get(0).(context.Context)
			gotRequestID = logging.RequestID(ctx)
			gotScanID = logging.ScanID(ctx)
		}).
		Return(notifier.BatchResult{Sent: 1})

	ctx := logging.WithScanID(logging.WithRequestID(s.T().Context(), "req-xyz"), "scan-xyz")
	s.client.SendReleaseNotifications(ctx, notifications)

	s.Equal("req-xyz", gotRequestID)
	s.Equal("scan-xyz", gotScanID)
}

func (s *EmailerGRPCSuite) TestSendConfirmation() {
	done := make(chan struct{})
	s.mailer.On("SendConfirmation", mock.Anything,
		"user@example.com", "owner/repo", "https://example.com/confirm/abc").
		Return().
		Run(func(mock.Arguments) { close(done) })

	s.client.SendConfirmation(s.T().Context(), "user@example.com", "owner/repo", "https://example.com/confirm/abc")

	select {
	case <-done:
	case <-time.After(2 * time.Second):
		s.Fail("confirmation was not delivered to the emailer")
	}
}
