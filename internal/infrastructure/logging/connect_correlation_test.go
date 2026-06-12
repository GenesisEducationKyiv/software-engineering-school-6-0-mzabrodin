package logging

import (
	"testing"

	"connectrpc.com/connect"
	"github.com/stretchr/testify/suite"
	"google.golang.org/protobuf/types/known/emptypb"
)

type CorrelationSuite struct {
	suite.Suite
}

func TestCorrelationSuite(t *testing.T) {
	suite.Run(t, new(CorrelationSuite))
}

func (s *CorrelationSuite) TestAttachOutgoingIDs() {
	ctx := WithScanID(WithRequestID(s.T().Context(), "req-1"), "scan-1")
	req := connect.NewRequest(&emptypb.Empty{})

	attachOutgoingIDs(ctx, req)

	s.Equal("req-1", req.Header().Get(requestIDMetaKey))
	s.Equal("scan-1", req.Header().Get(scanIDMetaKey))
}

func (s *CorrelationSuite) TestAttachOutgoingIDsSkipsEmpty() {
	req := connect.NewRequest(&emptypb.Empty{})

	attachOutgoingIDs(s.T().Context(), req)

	s.Empty(req.Header().Get(requestIDMetaKey))
	s.Empty(req.Header().Get(scanIDMetaKey))
}

func (s *CorrelationSuite) TestReadIncomingIDsFromHeaders() {
	req := connect.NewRequest(&emptypb.Empty{})
	req.Header().Set(requestIDMetaKey, "req-2")
	req.Header().Set(scanIDMetaKey, "scan-2")

	ctx := readIncomingIDs(s.T().Context(), req)

	s.Equal("req-2", RequestID(ctx))
	s.Equal("scan-2", ScanID(ctx))
}

func (s *CorrelationSuite) TestReadIncomingIDsGeneratesRequestID() {
	req := connect.NewRequest(&emptypb.Empty{})

	ctx := readIncomingIDs(s.T().Context(), req)

	s.NotEmpty(RequestID(ctx), "a request id is generated when none is incoming")
	s.Empty(ScanID(ctx), "scan id stays unset when none is incoming")
}
