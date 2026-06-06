package subscribe_test

import (
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/suite"

	"github-release-notifier/internal/entity"
	"github-release-notifier/internal/usecase/subscribe"
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
		setupMocks func(*mockRepoRepository, *mockSubRepository, *mockGitHub, *mockMailer)
		wantErrIs  error
		wantAnyErr bool
		check      func(*SubscribeSuite, error)
	}{
		{
			name:      "invalid email",
			email:     "not-an-email",
			repo:      "owner/repo",
			wantErrIs: entity.ErrInvalidEmail,
		},
		{
			name:      "invalid repo format",
			email:     "user@example.com",
			repo:      "invalid",
			wantErrIs: entity.ErrInvalidRepo,
		},
		{
			name:  "repo not found on github",
			email: "user@example.com",
			repo:  "owner/repo",
			setupMocks: func(repos *mockRepoRepository, subs *mockSubRepository, gh *mockGitHub, mailer *mockMailer) {
				gh.On("RepoExists", mock.Anything, "owner", "repo").Return(false, nil)
			},
			wantErrIs: entity.ErrRepoNotFound,
		},
		{
			name:  "github error",
			email: "user@example.com",
			repo:  "owner/repo",
			setupMocks: func(repos *mockRepoRepository, subs *mockSubRepository, gh *mockGitHub, mailer *mockMailer) {
				gh.On("RepoExists", mock.Anything, "owner", "repo").Return(false, entity.ErrRateLimited)
			},
			wantAnyErr: true,
			check: func(s *SubscribeSuite, err error) {
				s.NotErrorIs(err, entity.ErrRepoNotFound)
			},
		},
		{
			name:  "new repo creates and subscribes",
			email: "user@example.com",
			repo:  "owner/repo",
			setupMocks: func(repos *mockRepoRepository, subs *mockSubRepository, gh *mockGitHub, mailer *mockMailer) {
				gh.On("RepoExists", mock.Anything, "owner", "repo").Return(true, nil)
				repos.On("GetByName", mock.Anything, "owner/repo").Return(nil, entity.ErrNotFound)
				repos.On("Create", mock.Anything, mock.Anything).Return(nil)
				subs.On("Create", mock.Anything, mock.Anything).Return(nil)
				mailer.On("SendConfirmation", "user@example.com", "owner/repo", mock.Anything)
			},
		},
		{
			name:  "repo get error",
			email: "user@example.com",
			repo:  "owner/repo",
			setupMocks: func(repos *mockRepoRepository, subs *mockSubRepository, gh *mockGitHub, mailer *mockMailer) {
				gh.On("RepoExists", mock.Anything, "owner", "repo").Return(true, nil)
				repos.On("GetByName", mock.Anything, "owner/repo").Return(nil, assert.AnError)
			},
			wantAnyErr: true,
		},
		{
			name:  "existing repo skips create",
			email: "user@example.com",
			repo:  "owner/repo",
			setupMocks: func(repos *mockRepoRepository, subs *mockSubRepository, gh *mockGitHub, mailer *mockMailer) {
				existingRepo := &entity.Repository{ID: uuid.New(), Name: "owner/repo"}
				gh.On("RepoExists", mock.Anything, "owner", "repo").Return(true, nil)
				repos.On("GetByName", mock.Anything, "owner/repo").Return(existingRepo, nil)
				subs.On("Create", mock.Anything, mock.Anything).Return(nil)
				mailer.On("SendConfirmation", mock.Anything, mock.Anything, mock.Anything)
			},
		},
		{
			name:  "already subscribed",
			email: "user@example.com",
			repo:  "owner/repo",
			setupMocks: func(repos *mockRepoRepository, subs *mockSubRepository, gh *mockGitHub, mailer *mockMailer) {
				existingRepo := &entity.Repository{ID: uuid.New(), Name: "owner/repo"}
				gh.On("RepoExists", mock.Anything, "owner", "repo").Return(true, nil)
				repos.On("GetByName", mock.Anything, "owner/repo").Return(existingRepo, nil)
				subs.On("Create", mock.Anything, mock.Anything).Return(entity.ErrAlreadyExists)
			},
			wantErrIs: entity.ErrAlreadyExists,
		},
	}

	for _, tc := range cases {
		s.Run(tc.name, func() {
			repos := &mockRepoRepository{}
			subs := &mockSubRepository{}
			gh := &mockGitHub{}
			mailer := &mockMailer{}
			defer repos.AssertExpectations(s.T())
			defer subs.AssertExpectations(s.T())
			defer gh.AssertExpectations(s.T())
			defer mailer.AssertExpectations(s.T())

			if tc.setupMocks != nil {
				tc.setupMocks(repos, subs, gh, mailer)
			}

			uc := newUseCase(repos, subs, gh, mailer)
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
