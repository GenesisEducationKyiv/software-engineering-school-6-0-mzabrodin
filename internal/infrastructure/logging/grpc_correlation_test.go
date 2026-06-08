package logging_test

import (
	"context"
	"testing"

	"github.com/stretchr/testify/suite"
	"google.golang.org/grpc"
	"google.golang.org/grpc/metadata"

	"github-release-notifier/internal/infrastructure/logging"
)

type GRPCRequestIDSuite struct {
	suite.Suite
}

func TestGRPCRequestIDSuite(t *testing.T) {
	suite.Run(t, new(GRPCRequestIDSuite))
}

func (s *GRPCRequestIDSuite) invoke(md metadata.MD) (gotRequestID, gotScanID string, header metadata.MD) {
	s.T().Helper()

	stream := &stubTransportStream{}
	ctx := grpc.NewContextWithServerTransportStream(context.Background(), stream)
	if md != nil {
		ctx = metadata.NewIncomingContext(ctx, md)
	}

	handler := func(ctx context.Context, _ any) (any, error) {
		gotRequestID = logging.RequestID(ctx)
		gotScanID = logging.ScanID(ctx)

		return "ok", nil
	}

	_, err := logging.NewCorrelationInterceptor()(ctx, nil, &grpc.UnaryServerInfo{}, handler)
	s.Require().NoError(err)

	return gotRequestID, gotScanID, stream.header
}

func (s *GRPCRequestIDSuite) TestGeneratesRequestIDWhenAbsent() {
	requestID, scanID, header := s.invoke(nil)

	s.NotEmpty(requestID)
	s.Empty(scanID)
	s.Equal([]string{requestID}, header.Get("x-request-id"))
}

func (s *GRPCRequestIDSuite) TestHonorsIncomingRequestID() {
	requestID, _, header := s.invoke(metadata.Pairs("x-request-id", "req-abc"))

	s.Equal("req-abc", requestID)
	s.Equal([]string{"req-abc"}, header.Get("x-request-id"))
}

func (s *GRPCRequestIDSuite) TestPropagatesIncomingScanID() {
	requestID, scanID, _ := s.invoke(metadata.Pairs("x-scan-id", "scan-9"))

	s.NotEmpty(requestID)
	s.Equal("scan-9", scanID)
}

func (s *GRPCRequestIDSuite) TestWithOutgoingIDsAttachesBoth() {
	ctx := logging.WithScanID(logging.WithRequestID(context.Background(), "r1"), "s1")

	md, ok := metadata.FromOutgoingContext(logging.WithOutgoingIDs(ctx))
	s.Require().True(ok)
	s.Equal([]string{"r1"}, md.Get("x-request-id"))
	s.Equal([]string{"s1"}, md.Get("x-scan-id"))
}

func (s *GRPCRequestIDSuite) TestWithOutgoingIDsAttachesNothingWhenAbsent() {
	_, ok := metadata.FromOutgoingContext(logging.WithOutgoingIDs(context.Background()))
	s.False(ok)
}

// stubTransportStream records headers set via grpc.SetHeader so the test can assert the echoed id.
type stubTransportStream struct {
	header metadata.MD
}

func (s *stubTransportStream) Method() string { return "" }

func (s *stubTransportStream) SetHeader(md metadata.MD) error {
	s.header = metadata.Join(s.header, md)
	return nil
}

func (s *stubTransportStream) SendHeader(metadata.MD) error { return nil }

func (s *stubTransportStream) SetTrailer(metadata.MD) error { return nil }
