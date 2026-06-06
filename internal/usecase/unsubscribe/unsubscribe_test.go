package unsubscribe_test

import (
	"context"
	"io"
	"log/slog"
	"testing"

	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/suite"

	"github-release-notifier/internal/entity"
	"github-release-notifier/internal/usecase/unsubscribe"
)

var testLogger = slog.New(slog.NewTextHandler(io.Discard, nil))

type mockSubRepository struct{ mock.Mock }

func (m *mockSubRepository) Delete(ctx context.Context, token string) error {
	return m.Called(ctx, token).Error(0)
}

type UnsubscribeSuite struct {
	suite.Suite
}

func TestUnsubscribeSuite(t *testing.T) {
	suite.Run(t, new(UnsubscribeSuite))
}

func (s *UnsubscribeSuite) TestExecute() {
	cases := []struct {
		name      string
		mockErr   error
		wantErrIs error
	}{
		{"success", nil, nil},
		{"not found", entity.ErrNotFound, entity.ErrNotFound},
	}
	for _, tc := range cases {
		s.Run(tc.name, func() {
			subs := &mockSubRepository{}
			subs.On("Delete", mock.Anything, "sometoken").Return(tc.mockErr)
			defer subs.AssertExpectations(s.T())

			uc := unsubscribe.New(subs, testLogger)
			_, err := uc.Execute(s.T().Context(), unsubscribe.Input{Token: "sometoken"})
			if tc.wantErrIs != nil {
				s.ErrorIs(err, tc.wantErrIs)
			} else {
				s.NoError(err)
			}
		})
	}
}
