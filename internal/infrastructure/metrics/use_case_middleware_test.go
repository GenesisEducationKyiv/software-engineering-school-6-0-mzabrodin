package metrics_test

import (
	"errors"
	"testing"

	"github.com/prometheus/client_golang/prometheus/testutil"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/suite"

	"github-release-notifier/internal/infrastructure/metrics"
)

type MeteredSuite struct {
	suite.Suite
}

func TestMeteredSuite(t *testing.T) {
	suite.Run(t, new(MeteredSuite))
}

func (s *MeteredSuite) TestExecute() {
	cases := []struct {
		name    string
		op      string
		out     string
		mockErr error
		result  string
	}{
		{"success", "test_success", "result", nil, "success"},
		{"error", "test_error", "", errors.New("boom"), "error"},
	}
	for _, tc := range cases {
		s.Run(tc.name, func() {
			inner := &mockUseCase{}
			inner.On("Execute", mock.Anything, "input").Return(tc.out, tc.mockErr)
			defer inner.AssertExpectations(s.T())

			metered := metrics.NewMetered[string, string](tc.op, inner)
			out, err := metered.Execute(s.T().Context(), "input")

			s.Equal(tc.out, out)
			if tc.mockErr != nil {
				s.ErrorIs(err, tc.mockErr)
			} else {
				s.NoError(err)
			}

			s.InDelta(
				1,
				testutil.ToFloat64(metrics.SubscriptionOperationsTotal.WithLabelValues(tc.op, tc.result)),
				0.0001,
			)
		})
	}
}
