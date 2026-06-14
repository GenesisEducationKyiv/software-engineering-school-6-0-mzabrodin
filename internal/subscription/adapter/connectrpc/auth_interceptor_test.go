package connectapi

import (
	"context"
	"testing"

	"connectrpc.com/connect"
	"github.com/stretchr/testify/suite"

	"github-release-notifier/internal/subscription/grpc/gen/subscriptionv1"
)

type AuthInterceptorSuite struct {
	suite.Suite
}

func TestAuthInterceptorSuite(t *testing.T) {
	suite.Run(t, new(AuthInterceptorSuite))
}

func (s *AuthInterceptorSuite) TestBearerToken() {
	cases := []struct {
		name   string
		header string
		want   string
	}{
		{"canonical scheme", "Bearer secret", "secret"},
		{"lowercase scheme", "bearer secret", "secret"},
		{"uppercase scheme", "BEARER secret", "secret"},
		{"empty header", "", ""},
		{"no scheme", "secret", ""},
		{"wrong scheme", "Token secret", ""},
		{"scheme only", "Bearer ", ""},
	}

	for _, tc := range cases {
		s.Run(tc.name, func() {
			s.Equal(tc.want, bearerToken(tc.header))
		})
	}
}

func (s *AuthInterceptorSuite) TestEmptyAPIKeyPassesThrough() {
	called := false
	next := func(ctx context.Context, _ connect.AnyRequest) (connect.AnyResponse, error) {
		called = true
		return connect.NewResponse(&subscriptionv1.SubscribeResponse{}), nil
	}

	wrapped := NewAuthInterceptor("")(next)
	_, err := wrapped(s.T().Context(), connect.NewRequest(&subscriptionv1.SubscribeRequest{}))

	s.Require().NoError(err)
	s.True(called, "no auth is enforced when the API key is unset")
}

func (s *AuthInterceptorSuite) TestUnprotectedProcedurePassesThrough() {
	called := false
	next := func(ctx context.Context, _ connect.AnyRequest) (connect.AnyResponse, error) {
		called = true
		return connect.NewResponse(&subscriptionv1.SubscribeResponse{}), nil
	}

	wrapped := NewAuthInterceptor("secret")(next)
	_, err := wrapped(s.T().Context(), connect.NewRequest(&subscriptionv1.SubscribeRequest{}))

	s.Require().NoError(err)
	s.True(called)
}
