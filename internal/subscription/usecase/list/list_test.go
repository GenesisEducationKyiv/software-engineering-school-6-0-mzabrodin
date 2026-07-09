package list_test

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/suite"

	"github-release-notifier/internal/subscription/domain"
	"github-release-notifier/internal/subscription/usecase/list"
)

type ListSuite struct {
	suite.Suite
}

func TestListSuite(t *testing.T) {
	suite.Run(t, new(ListSuite))
}

func (s *ListSuite) TestExecute() {
	cases := []struct {
		name      string
		email     string
		setupMock func(mocks)
		wantLen   int
		wantErr   bool
	}{
		{
			name:  "returns list",
			email: "user@example.com",
			setupMock: func(m mocks) {
				views := []domain.SubscriptionView{
					{Email: "user@example.com", Repo: "owner/repo", Confirmed: true},
				}
				m.subs.On("GetByEmail", mock.Anything, "user@example.com").Return(views, nil)
			},
			wantLen: 1,
		},
		{
			name:  "repository error",
			email: "user@example.com",
			setupMock: func(m mocks) {
				m.subs.On("GetByEmail", mock.Anything, "user@example.com").Return(nil, assert.AnError)
			},
			wantErr: true,
		},
	}

	for _, tc := range cases {
		s.Run(tc.name, func() {
			m := newMocks()
			defer m.assertExpectations(s.T())
			if tc.setupMock != nil {
				tc.setupMock(m)
			}

			out, err := m.useCase().Execute(s.T().Context(), list.Input{Email: tc.email})
			switch {
			case tc.wantErr:
				s.Error(err)
			default:
				s.Require().NoError(err)
				s.Len(out.Views, tc.wantLen)
			}
		})
	}
}
