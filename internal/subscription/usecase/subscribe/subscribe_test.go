package subscribe_test

import (
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/suite"

	shareddomain "github-release-notifier/internal/shared/domain"
	"github-release-notifier/internal/shared/events"
	"github-release-notifier/internal/shared/github"
	"github-release-notifier/internal/subscription/domain"
	"github-release-notifier/internal/subscription/usecase/subscribe"
)

type SubscribeSuite struct {
	suite.Suite
}

func TestSubscribeSuite(t *testing.T) {
	suite.Run(t, new(SubscribeSuite))
}

func (s *SubscribeSuite) TestExecute() {
	cases := []struct {
		name       string
		email      string
		repo       string
		setupMocks func(mocks)
		wantErrIs  error
		wantAnyErr bool
		check      func(*SubscribeSuite, error)
	}{
		{
			name:      "invalid repo format",
			email:     "user@example.com",
			repo:      "invalid",
			wantErrIs: github.ErrInvalidRepo,
		},
		{
			name:  "repo not found on github",
			email: "user@example.com",
			repo:  "owner/repo",
			setupMocks: func(m mocks) {
				m.gh.On("RepoExists", mock.Anything, "owner", "repo").Return(false, nil)
			},
			wantErrIs: github.ErrRepoNotFound,
		},
		{
			name:  "github error",
			email: "user@example.com",
			repo:  "owner/repo",
			setupMocks: func(m mocks) {
				m.gh.On("RepoExists", mock.Anything, "owner", "repo").Return(false, github.ErrRateLimited)
			},
			wantAnyErr: true,
			check: func(s *SubscribeSuite, err error) {
				s.NotErrorIs(err, github.ErrRepoNotFound)
			},
		},
		{
			name:  "ensure repository error",
			email: "user@example.com",
			repo:  "owner/repo",
			setupMocks: func(m mocks) {
				m.gh.On("RepoExists", mock.Anything, "owner", "repo").Return(true, nil)
				m.repos.On("GetOrCreate", mock.Anything, "owner/repo").
					Return(domain.Repository{}, assert.AnError)
			},
			wantAnyErr: true,
		},
		{
			name:  "new subscriber creates and publishes pending",
			email: "user@example.com",
			repo:  "owner/repo",
			setupMocks: func(m mocks) {
				repoID := uuid.New()
				m.expectRepoResolved(repoID)
				m.subs.On("FindByEmailAndRepo", mock.Anything, "user@example.com", repoID).
					Return(domain.Subscription{}, shareddomain.ErrNotFound)
				m.tokens.On("Issue", "user@example.com", "owner/repo").Return("jwt-token", nil)
				m.urls.On("ConfirmURL", "jwt-token").Return("http://localhost:8080/api/confirm/jwt-token")
				m.tx.On("Within", mock.Anything).Return(nil)
				m.subs.On("Create", mock.Anything, mock.Anything).Return(nil)
				m.pub.On("SubscriptionPending", mock.Anything, mock.MatchedBy(func(ev events.SubscriptionPending) bool {
					return ev.Email == "user@example.com" &&
						ev.RepoName == "owner/repo" &&
						ev.ConfirmURL == "http://localhost:8080/api/confirm/jwt-token" &&
						ev.SagaID != ""
				})).Return(nil)
				m.pub.On("Notify").Return()
			},
		},
		{
			name:  "resubscribe pending republishes without create",
			email: "user@example.com",
			repo:  "owner/repo",
			setupMocks: func(m mocks) {
				repoID := uuid.New()
				m.expectRepoResolved(repoID)
				m.subs.On("FindByEmailAndRepo", mock.Anything, "user@example.com", repoID).
					Return(domain.Subscription{RepositoryID: repoID, Email: "user@example.com", Confirmed: false}, nil)
				m.tokens.On("Issue", "user@example.com", "owner/repo").Return("jwt-token", nil)
				m.urls.On("ConfirmURL", "jwt-token").Return("http://localhost:8080/api/confirm/jwt-token")
				m.tx.On("Within", mock.Anything).Return(nil)
				m.pub.On("SubscriptionPending", mock.Anything, mock.Anything).Return(nil)
				m.pub.On("Notify").Return()
			},
		},
		{
			name:  "already confirmed returns already exists",
			email: "user@example.com",
			repo:  "owner/repo",
			setupMocks: func(m mocks) {
				repoID := uuid.New()
				m.expectRepoResolved(repoID)
				m.subs.On("FindByEmailAndRepo", mock.Anything, "user@example.com", repoID).
					Return(domain.Subscription{RepositoryID: repoID, Email: "user@example.com", Confirmed: true}, nil)
			},
			wantErrIs: shareddomain.ErrAlreadyExists,
		},
	}

	for _, tc := range cases {
		s.Run(tc.name, func() {
			m := newMocks()
			defer m.assertExpectations(s.T())

			if tc.setupMocks != nil {
				tc.setupMocks(m)
			}

			uc := m.useCase()
			_, err := uc.Execute(s.T().Context(), subscribe.Input{Email: tc.email, Repo: tc.repo})
			switch {
			case tc.wantErrIs != nil:
				s.ErrorIs(err, tc.wantErrIs)
			case tc.wantAnyErr:
				s.Error(err)
			default:
				s.NoError(err)
			}
			if tc.check != nil {
				tc.check(s, err)
			}
		})
	}
}
