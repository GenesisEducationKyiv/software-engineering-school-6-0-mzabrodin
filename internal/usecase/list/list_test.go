package list_test

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/suite"

	"github-release-notifier/internal/entity"
	"github-release-notifier/internal/usecase/list"
)

type ListSuite struct {
	suite.Suite
}

func TestListSuite(t *testing.T) {
	suite.Run(t, new(ListSuite))
}

func (s *ListSuite) TestExecute() {
	tag := "v1.0.0"
	cases := []struct {
		name      string
		email     string
		setupMock func(*mockSubRepository)
		wantLen   int
		wantErrIs error
		wantErr   bool
	}{
		{
			name:      "invalid email",
			email:     "not-an-email",
			wantErrIs: entity.ErrInvalidEmail,
		},
		{
			name:  "returns list",
			email: "user@example.com",
			setupMock: func(subs *mockSubRepository) {
				views := []*entity.SubscriptionView{
					{Email: "user@example.com", Repo: "owner/repo", Confirmed: true, LastSeenTag: &tag},
				}
				subs.On("GetByEmail", mock.Anything, "user@example.com").Return(views, nil)
			},
			wantLen: 1,
		},
		{
			name:  "repository error",
			email: "user@example.com",
			setupMock: func(subs *mockSubRepository) {
				subs.On("GetByEmail", mock.Anything, "user@example.com").Return(nil, assert.AnError)
			},
			wantErr: true,
		},
	}

	for _, tc := range cases {
		s.Run(tc.name, func() {
			subs := &mockSubRepository{}
			defer subs.AssertExpectations(s.T())
			if tc.setupMock != nil {
				tc.setupMock(subs)
			}

			uc := list.New(subs)
			out, err := uc.Execute(s.T().Context(), list.Input{Email: tc.email})
			switch {
			case tc.wantErrIs != nil:
				s.ErrorIs(err, tc.wantErrIs)
			case tc.wantErr:
				s.Error(err)
			default:
				s.Require().NoError(err)
				s.Len(out.Views, tc.wantLen)
			}
		})
	}
}
