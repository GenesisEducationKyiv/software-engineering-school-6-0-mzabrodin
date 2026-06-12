package metrics_test

import (
	"context"
	"errors"
	"io"
	"log/slog"
	"testing"

	"connectrpc.com/connect"
	"github.com/prometheus/client_golang/prometheus/testutil"
	"github.com/stretchr/testify/suite"
	"google.golang.org/protobuf/types/known/emptypb"

	"github-release-notifier/internal/infrastructure/metrics"
)

type ObservabilitySuite struct {
	suite.Suite
	log *slog.Logger
}

func TestObservabilitySuite(t *testing.T) {
	suite.Run(t, new(ObservabilitySuite))
}

func (s *ObservabilitySuite) SetupTest() {
	s.log = slog.New(slog.NewTextHandler(io.Discard, nil))
}

// invoke runs the observability interceptor once over a stub handler that
// returns nextErr, using the given protocol resolver.
func (s *ObservabilitySuite) invoke(protocol func(context.Context) string, nextErr error) {
	next := func(_ context.Context, _ connect.AnyRequest) (connect.AnyResponse, error) {
		if nextErr != nil {
			return nil, nextErr
		}
		return connect.NewResponse(&emptypb.Empty{}), nil
	}

	wrapped := metrics.NewConnectObservabilityInterceptor(s.log, protocol)(next)
	_, err := wrapped(s.T().Context(), connect.NewRequest(&emptypb.Empty{}))

	if nextErr != nil {
		s.Require().Error(err)
	} else {
		s.Require().NoError(err)
	}
}

func (s *ObservabilitySuite) TestRecordsSuccess() {
	s.invoke(func(context.Context) string { return "obs-ok" }, nil)

	s.InDelta(1, testutil.ToFloat64(metrics.RequestsTotal.WithLabelValues("obs-ok", "", "ok")), 0.0001)
}

func (s *ObservabilitySuite) TestRecordsErrorCode() {
	s.invoke(
		func(context.Context) string { return "obs-err" },
		connect.NewError(connect.CodeInvalidArgument, errors.New("bad")),
	)

	s.InDelta(
		1,
		testutil.ToFloat64(metrics.RequestsTotal.WithLabelValues("obs-err", "", "invalid_argument")),
		0.0001,
	)
}

func (s *ObservabilitySuite) TestUnknownProtocolFallback() {
	s.invoke(func(context.Context) string { return "" }, nil)

	s.InDelta(1, testutil.ToFloat64(metrics.RequestsTotal.WithLabelValues("unknown", "", "ok")), 0.0001)
}
