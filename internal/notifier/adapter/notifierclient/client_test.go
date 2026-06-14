package notifierclient

import (
	"context"
	"io"
	"log/slog"
	"testing"

	"connectrpc.com/connect"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/suite"

	"github-release-notifier/internal/notifier"
	"github-release-notifier/internal/notifier/grpc/gen/notifierv1"
)

var testLogger = slog.New(slog.NewTextHandler(io.Discard, nil))

type mockRPC struct{ mock.Mock }

func (m *mockRPC) SendConfirmation(
	ctx context.Context,
	req *connect.Request[notifierv1.SendConfirmationRequest],
) (*connect.Response[notifierv1.SendConfirmationResponse], error) {
	args := m.Called(ctx, req.Msg)
	out, _ := args.Get(0).(*notifierv1.SendConfirmationResponse)
	if out == nil {
		return nil, args.Error(1)
	}
	return connect.NewResponse(out), args.Error(1)
}

func (m *mockRPC) SendReleaseNotifications(
	ctx context.Context,
	req *connect.Request[notifierv1.SendReleaseNotificationsRequest],
) (*connect.Response[notifierv1.SendReleaseNotificationsResponse], error) {
	args := m.Called(ctx, req.Msg)
	out, _ := args.Get(0).(*notifierv1.SendReleaseNotificationsResponse)
	if out == nil {
		return nil, args.Error(1)
	}
	return connect.NewResponse(out), args.Error(1)
}

type ClientSuite struct {
	suite.Suite

	rpc    *mockRPC
	client *Client
}

func TestClientSuite(t *testing.T) {
	suite.Run(t, new(ClientSuite))
}

func (s *ClientSuite) SetupTest() {
	s.rpc = &mockRPC{}
	s.client = newClient(s.rpc, nil, testLogger)
}

func (s *ClientSuite) TearDownTest() {
	s.rpc.AssertExpectations(s.T())
}

func notification(to string) notifier.ReleaseNotification {
	return notifier.ReleaseNotification{
		To:             to,
		Repo:           "owner/repo",
		Tag:            "v1.0.0",
		ReleaseURL:     "https://github.com/owner/repo/releases/tag/v1.0.0",
		UnsubscribeURL: "https://example.com/unsubscribe/" + to,
	}
}

func (s *ClientSuite) TestSendConfirmation() {
	s.rpc.On("SendConfirmation", mock.Anything, mock.MatchedBy(func(req *notifierv1.SendConfirmationRequest) bool {
		return req.GetTo() == "user@example.com" &&
			req.GetRepo() == "owner/repo" &&
			req.GetConfirmUrl() == "https://example.com/confirm/abc"
	})).Return(&notifierv1.SendConfirmationResponse{}, nil)

	s.client.SendConfirmation(s.T().Context(), "user@example.com", "owner/repo", "https://example.com/confirm/abc")

	s.Require().NoError(s.client.Close())
}

func (s *ClientSuite) TestSendConfirmationSwallowsError() {
	s.rpc.On("SendConfirmation", mock.Anything, mock.Anything).
		Return(nil, assert.AnError)

	s.client.SendConfirmation(s.T().Context(), "user@example.com", "owner/repo", "https://example.com/confirm/abc")

	s.Require().NoError(s.client.Close())
}

func (s *ClientSuite) TestSendReleaseNotificationsSuccess() {
	s.rpc.On("SendReleaseNotifications", mock.Anything,
		mock.MatchedBy(func(req *notifierv1.SendReleaseNotificationsRequest) bool {
			return len(req.GetNotifications()) == 2 &&
				req.GetNotifications()[0].GetTo() == "a@example.com"
		})).
		Return(&notifierv1.SendReleaseNotificationsResponse{Sent: 2, Failed: []string{"b@example.com"}}, nil)

	result := s.client.SendReleaseNotifications(s.T().Context(), []notifier.ReleaseNotification{
		notification("a@example.com"),
		notification("b@example.com"),
	})

	s.Equal(2, result.Sent)
	s.Equal([]string{"b@example.com"}, result.Failed)
}

func (s *ClientSuite) TestSendReleaseNotificationsTransportError() {
	s.rpc.On("SendReleaseNotifications", mock.Anything, mock.Anything).
		Return(nil, assert.AnError)

	result := s.client.SendReleaseNotifications(s.T().Context(), []notifier.ReleaseNotification{
		notification("a@example.com"),
		notification("b@example.com"),
	})

	s.Equal(0, result.Sent)
	s.Equal([]string{"a@example.com", "b@example.com"}, result.Failed)
}

func (s *ClientSuite) TestCloseNilConn() {
	s.Require().NoError(s.client.Close())
}
