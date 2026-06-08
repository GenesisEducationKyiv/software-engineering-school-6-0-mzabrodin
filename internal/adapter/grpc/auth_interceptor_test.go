package grcp_api_test

import (
	"context"
	"testing"

	"github.com/stretchr/testify/suite"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/metadata"
	"google.golang.org/grpc/status"

	grpcapi "github-release-notifier/internal/adapter/grpc"
	notifierv1 "github-release-notifier/internal/adapter/grpc/gen/app/v1"
)

type KeyAuthInterceptorSuite struct {
	suite.Suite
}

func TestKeyAuthInterceptorSuite(t *testing.T) {
	suite.Run(t, new(KeyAuthInterceptorSuite))
}

func (s *KeyAuthInterceptorSuite) TestKeyAuthInterceptor() {
	const protected = notifierv1.SubscriptionService_Subscribe_FullMethodName
	const unprotected = notifierv1.SubscriptionService_Confirm_FullMethodName

	cases := []struct {
		name      string
		key       string
		method    string
		setHeader bool
		headerKey string
		wantCode  codes.Code
	}{
		{"no key configured allows protected", "", protected, false, "", codes.OK},
		{"correct key allows protected", "secret", protected, true, "secret", codes.OK},
		{"wrong key returns unauthenticated", "secret", protected, true, "wrong", codes.Unauthenticated},
		{"missing metadata returns unauthenticated", "secret", protected, false, "", codes.Unauthenticated},
		{"unprotected method skips auth", "secret", unprotected, false, "", codes.OK},
	}

	for _, tc := range cases {
		s.Run(tc.name, func() {
			interceptor := grpcapi.NewKeyAuthInterceptor(tc.key)

			ctx := context.Background()
			if tc.setHeader {
				ctx = metadata.NewIncomingContext(ctx, metadata.Pairs("x-api-key", tc.headerKey))
			}

			handlerCalled := false
			handler := func(_ context.Context, _ any) (any, error) {
				handlerCalled = true
				return struct{}{}, nil
			}

			_, err := interceptor(ctx, struct{}{}, &grpc.UnaryServerInfo{FullMethod: tc.method}, handler)

			s.Equal(tc.wantCode, status.Code(err))
			s.Equal(tc.wantCode == codes.OK, handlerCalled)
		})
	}
}
