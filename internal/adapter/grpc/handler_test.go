package grcp_api_test

import (
	"io"
	"log/slog"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/suite"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	grpcapi "github-release-notifier/internal/adapter/grpc"
	notifierv1 "github-release-notifier/internal/adapter/grpc/gen/app/v1"
	"github-release-notifier/internal/entity"
	"github-release-notifier/internal/usecase/confirm"
	"github-release-notifier/internal/usecase/list"
	"github-release-notifier/internal/usecase/subscribe"
	"github-release-notifier/internal/usecase/unsubscribe"
)

// 64 hex chars
const validToken = "7453d94668d17cf6adfc2b37045347fa14907a007786ed791865d1754b5737f6"

type ServerSuite struct {
	suite.Suite
	server      *grpcapi.Server
	subscribe   *mockSubscribe
	confirm     *mockConfirm
	unsubscribe *mockUnsubscribe
	list        *mockList
}

func (s *ServerSuite) SetupSubTest() {
	s.subscribe = &mockSubscribe{}
	s.confirm = &mockConfirm{}
	s.unsubscribe = &mockUnsubscribe{}
	s.list = &mockList{}
	s.server = grpcapi.NewServer(
		s.subscribe,
		s.confirm,
		s.unsubscribe,
		s.list,
		slog.New(slog.NewTextHandler(io.Discard, nil)),
	)
}

func (s *ServerSuite) TearDownSubTest() {
	s.subscribe.AssertExpectations(s.T())
	s.confirm.AssertExpectations(s.T())
	s.unsubscribe.AssertExpectations(s.T())
	s.list.AssertExpectations(s.T())
}

func TestServerSuite(t *testing.T) {
	suite.Run(t, new(ServerSuite))
}

func (s *ServerSuite) TestSubscribe() {
	in := subscribe.Input{Email: "user@example.com", Repo: "owner/repo"}

	cases := []struct {
		name      string
		returnErr error
		wantCode  codes.Code
	}{
		{"success", nil, codes.OK},
		{"repo not found", entity.ErrRepoNotFound, codes.NotFound},
		{"already exists", entity.ErrAlreadyExists, codes.AlreadyExists},
		{"invalid repo", entity.ErrInvalidRepo, codes.InvalidArgument},
		{"internal error", assert.AnError, codes.Internal},
	}

	for _, tc := range cases {
		s.Run(tc.name, func() {
			s.subscribe.On("Execute", mock.Anything, in).Return(subscribe.Output{}, tc.returnErr)

			_, err := s.server.Subscribe(s.T().Context(), &notifierv1.SubscribeRequest{
				Email: in.Email,
				Repo:  in.Repo,
			})

			s.Equal(tc.wantCode, status.Code(err))
		})
	}
}

func (s *ServerSuite) TestConfirm() {
	cases := []struct {
		name      string
		returnErr error
		wantCode  codes.Code
	}{
		{"success", nil, codes.OK},
		{"not found", entity.ErrNotFound, codes.NotFound},
		{"internal error", assert.AnError, codes.Internal},
	}

	for _, tc := range cases {
		s.Run(tc.name, func() {
			s.confirm.On("Execute", mock.Anything, confirm.Input{Token: validToken}).
				Return(confirm.Output{}, tc.returnErr)

			_, err := s.server.Confirm(s.T().Context(), &notifierv1.ConfirmRequest{Token: validToken})

			s.Equal(tc.wantCode, status.Code(err))
		})
	}
}

func (s *ServerSuite) TestUnsubscribe() {
	cases := []struct {
		name      string
		returnErr error
		wantCode  codes.Code
	}{
		{"success", nil, codes.OK},
		{"not found", entity.ErrNotFound, codes.NotFound},
		{"internal error", assert.AnError, codes.Internal},
	}

	for _, tc := range cases {
		s.Run(tc.name, func() {
			s.unsubscribe.On("Execute", mock.Anything, unsubscribe.Input{Token: validToken}).
				Return(unsubscribe.Output{}, tc.returnErr)

			_, err := s.server.Unsubscribe(s.T().Context(), &notifierv1.UnsubscribeRequest{Token: validToken})

			s.Equal(tc.wantCode, status.Code(err))
		})
	}
}

func (s *ServerSuite) TestListSubscriptions() {
	s.Run("returns subscriptions", func() {
		tag := "v1.2.3"
		views := []*entity.SubscriptionView{
			{Email: "user@example.com", Repo: "owner/repo", Confirmed: true, LastSeenTag: &tag},
			{Email: "user@example.com", Repo: "owner/other", Confirmed: false},
		}
		s.list.On("Execute", mock.Anything, list.Input{Email: "user@example.com"}).
			Return(list.Output{Views: views}, nil)

		resp, err := s.server.ListSubscriptions(s.T().Context(), &notifierv1.ListSubscriptionsRequest{
			Email: "user@example.com",
		})

		s.Require().NoError(err)
		s.Require().Len(resp.GetSubscriptions(), 2)
		s.Equal("owner/repo", resp.GetSubscriptions()[0].GetRepo())
		s.Equal(tag, resp.GetSubscriptions()[0].GetLastSeenTag())
		s.False(resp.GetSubscriptions()[1].GetConfirmed())
		s.Empty(resp.GetSubscriptions()[1].GetLastSeenTag())
	})

	s.Run("internal error", func() {
		s.list.On("Execute", mock.Anything, list.Input{Email: "user@example.com"}).
			Return(list.Output{}, assert.AnError)

		_, err := s.server.ListSubscriptions(s.T().Context(), &notifierv1.ListSubscriptionsRequest{
			Email: "user@example.com",
		})

		s.Equal(codes.Internal, status.Code(err))
	})
}
