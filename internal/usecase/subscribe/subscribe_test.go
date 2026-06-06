package subscribe_test

import (
	"context"
	"io"
	"log/slog"
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/suite"

	"github-release-notifier/internal/entity"
	"github-release-notifier/internal/usecase/subscribe"
)

var testLogger = slog.New(slog.NewTextHandler(io.Discard, nil))

type mockRepoRepository struct{ mock.Mock }

func (m *mockRepoRepository) Create(ctx context.Context, repo *entity.Repository) error {
	return m.Called(ctx, repo).Error(0)
}

func (m *mockRepoRepository) GetByName(ctx context.Context, name string) (*entity.Repository, error) {
	args := m.Called(ctx, name)
	v, _ := args.Get(0).(*entity.Repository)
	return v, args.Error(1)
}

type mockSubRepository struct{ mock.Mock }

func (m *mockSubRepository) Create(ctx context.Context, sub *entity.Subscription) error {
	return m.Called(ctx, sub).Error(0)
}

type mockGitHub struct{ mock.Mock }

func (m *mockGitHub) RepoExists(ctx context.Context, owner, repo string) (bool, error) {
	args := m.Called(ctx, owner, repo)
	return args.Bool(0), args.Error(1)
}

type mockMailer struct{ mock.Mock }

func (m *mockMailer) SendConfirmation(_ context.Context, to, repo, confirmURL string) {
	m.Called(to, repo, confirmURL)
}

type mockURLBuilder struct{}

func (m *mockURLBuilder) ConfirmURL(token string) string {
	return "http://localhost:8080/api/confirm/" + token
}

func newUseCase(
	repos *mockRepoRepository,
	subs *mockSubRepository,
	gh *mockGitHub,
	mailer *mockMailer,
) *subscribe.UseCase {
	return subscribe.New(repos, subs, gh, mailer, &mockURLBuilder{}, testLogger)
}

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
