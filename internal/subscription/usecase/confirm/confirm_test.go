package confirm_test

import (
	"testing"

	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/suite"

	"github-release-notifier/internal/shared/entity"
	"github-release-notifier/internal/subscription/usecase/confirm"
)

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
