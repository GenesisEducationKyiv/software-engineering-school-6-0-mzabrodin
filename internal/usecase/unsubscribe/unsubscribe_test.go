package unsubscribe_test

import (
	"testing"

	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/suite"

	"github-release-notifier/internal/entity"
	"github-release-notifier/internal/usecase/unsubscribe"
)

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
