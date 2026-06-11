package grcp_api_test

import (
	"context"
	"testing"

	"buf.build/go/protovalidate"
	"github.com/stretchr/testify/suite"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/proto"

	grpcapi "github-release-notifier/internal/subscription/adapter/grpc"
	"github-release-notifier/internal/subscription/grpc/gen/appv1"
)

type ValidationInterceptorSuite struct {
	suite.Suite
}

func TestValidationInterceptorSuite(t *testing.T) {
	suite.Run(t, new(ValidationInterceptorSuite))
}

func (s *ValidationInterceptorSuite) TestValidationInterceptor() {
	const longToken = "7453d94668d17cf6adfc2b37045347fa14907a007786ed791865d1754b5737f6"

	cases := []struct {
		name     string
		req      proto.Message
		wantCode codes.Code
	}{
		{
			"valid subscribe",
			&appv1.SubscribeRequest{Email: "user@example.com", Repo: "owner/repo"},
			codes.OK,
		},
		{
			"invalid email",
			&appv1.SubscribeRequest{Email: "notanemail", Repo: "owner/repo"},
			codes.InvalidArgument,
		},
		{
			"invalid repo",
			&appv1.SubscribeRequest{Email: "user@example.com", Repo: "noslash"},
			codes.InvalidArgument,
		},
		{
			"valid token",
			&appv1.ConfirmRequest{Token: longToken},
			codes.OK,
		},
		{
			"short token",
			&appv1.ConfirmRequest{Token: "tooshort"},
			codes.InvalidArgument,
		},
	}

	validator, err := protovalidate.New()
	s.Require().NoError(err)

	interceptor := grpcapi.NewValidationInterceptor(validator)

	for _, tc := range cases {
		s.Run(tc.name, func() {
			handlerCalled := false
			handler := func(_ context.Context, _ any) (any, error) {
				handlerCalled = true
				return struct{}{}, nil
			}

			_, err := interceptor(
				context.Background(),
				tc.req,
				&grpc.UnaryServerInfo{},
				handler,
			)

			s.Equal(tc.wantCode, status.Code(err))
			s.Equal(tc.wantCode == codes.OK, handlerCalled)
		})
	}
}
