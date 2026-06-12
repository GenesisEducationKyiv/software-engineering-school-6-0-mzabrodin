package emailerserver

import (
	"context"
	"io"
	"log/slog"
	"testing"

	"connectrpc.com/connect"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/suite"

	"github-release-notifier/internal/notifier"
	"github-release-notifier/internal/notifier/grpc/gen/emailerv1"
)

var testLogger = slog.New(slog.NewTextHandler(io.Discard, nil))

type mockMailer struct{ mock.Mock }

func (m *mockMailer) SendConfirmation(ctx context.Context, to, repo, confirmURL string) {
	m.Called(ctx, to, repo, confirmURL)
}

func (m *mockMailer) SendReleaseNotifications(
	ctx context.Context,
	notifications []notifier.ReleaseNotification,
) notifier.BatchResult {
	args := m.Called(ctx, notifications)
	out, _ := args.Get(0).(notifier.BatchResult)
	return out
}

type HandlerSuite struct {
	suite.Suite

	mailer *mockMailer
	server *Server
}

func TestHandlerSuite(t *testing.T) {
	suite.Run(t, new(HandlerSuite))
}

func (s *HandlerSuite) SetupTest() {
	s.mailer = &mockMailer{}
	s.server = NewServer(s.mailer, testLogger)
}

func (s *HandlerSuite) TearDownTest() {
	s.mailer.AssertExpectations(s.T())
}

func (s *HandlerSuite) TestSendConfirmation() {
	s.mailer.On("SendConfirmation", mock.Anything,
		"user@example.com", "owner/repo", "https://example.com/confirm/abc").Return()

	resp, err := s.server.SendConfirmation(s.T().Context(), connect.NewRequest(&emailerv1.SendConfirmationRequest{
		To:         "user@example.com",
		Repo:       "owner/repo",
		ConfirmUrl: "https://example.com/confirm/abc",
	}))

	s.Require().NoError(err)
	s.NotNil(resp)
}

func (s *HandlerSuite) TestSendReleaseNotifications() {
	expected := []notifier.ReleaseNotification{{
		To:             "a@example.com",
		Repo:           "owner/repo",
		Tag:            "v1.0.0",
		ReleaseURL:     "https://github.com/owner/repo/releases/tag/v1.0.0",
		UnsubscribeURL: "https://example.com/unsubscribe/a",
	}}
	s.mailer.On("SendReleaseNotifications", mock.Anything, expected).
		Return(notifier.BatchResult{Sent: 2, Failed: []string{"b@example.com"}})

	resp, err := s.server.SendReleaseNotifications(
		s.T().Context(),
		connect.NewRequest(&emailerv1.SendReleaseNotificationsRequest{
			Notifications: []*emailerv1.ReleaseNotification{{
				To:             "a@example.com",
				Repo:           "owner/repo",
				Tag:            "v1.0.0",
				ReleaseUrl:     "https://github.com/owner/repo/releases/tag/v1.0.0",
				UnsubscribeUrl: "https://example.com/unsubscribe/a",
			}},
		}),
	)

	s.Require().NoError(err)
	s.Equal(uint32(2), resp.Msg.GetSent())
	s.Equal([]string{"b@example.com"}, resp.Msg.GetFailed())
}

func (s *HandlerSuite) TestNewHandler() {
	handler, err := NewHandler(s.server, testLogger)
	s.Require().NoError(err)
	s.NotNil(handler)
}
