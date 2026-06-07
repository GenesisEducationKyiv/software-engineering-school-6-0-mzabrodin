//go:build integration

package integration

import (
	"testing"

	"github.com/stretchr/testify/suite"
	healthpb "google.golang.org/grpc/health/grpc_health_v1"
)

type GRPCHealthSuite struct {
	suite.Suite
	health healthpb.HealthClient
}

func TestGRPCHealthSuite(t *testing.T) {
	suite.Run(t, new(GRPCHealthSuite))
}

func (s *GRPCHealthSuite) SetupTest() {
	truncateAll(s.T())
	s.health = healthpb.NewHealthClient(newTestGRPCConn(s.T(), true))
}

func (s *GRPCHealthSuite) TestServing() {
	resp, err := s.health.Check(s.T().Context(), &healthpb.HealthCheckRequest{})
	s.Require().NoError(err)
	s.Equal(healthpb.HealthCheckResponse_SERVING, resp.GetStatus())
}
