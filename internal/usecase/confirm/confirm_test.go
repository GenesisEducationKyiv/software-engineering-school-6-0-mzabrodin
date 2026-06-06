package confirm_test

import (
	"context"
	"io"
	"log/slog"
	"testing"

	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/suite"

	"github-release-notifier/internal/entity"
	"github-release-notifier/internal/usecase/confirm"
)

var testLogger = slog.New(slog.NewTextHandler(io.Discard, nil))

type mockSubRepository struct{ mock.Mock }

func (m *mockSubRepository) Confirm(ctx context.Context, token string) error {
	return m.Called(ctx, token).Error(0)
}

type ConfirmSuite struct {
	suite.Suite
}

func TestConfirmSuite(t *testing.T) {
	suite.Run(t, new(ConfirmSuite))
}

func (s *ConfirmSuite) TestExecute() {
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
			subs.On("Confirm", mock.Anything, "sometoken").Return(tc.mockErr)
			defer subs.AssertExpectations(s.T())

			uc := confirm.New(subs, testLogger)
			_, err := uc.Execute(s.T().Context(), confirm.Input{Token: "sometoken"})
			if tc.wantErrIs != nil {
				s.ErrorIs(err, tc.wantErrIs)
			} else {
				s.NoError(err)
			}
		})
	}
}
