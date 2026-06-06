package service_test

import (
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/suite"

	"github-release-notifier/internal/entity"
	"github-release-notifier/internal/service"
)

func newSvc(
	repos *mockRepoRepository,
	subs *mockSubRepository,
	gh *mockGitHub,
	mailer *mockMailer,
) *service.SubscriptionService {
	return service.NewSubscriptionService(repos, subs, gh, mailer, &mockURLBuilder{}, testLogger)
}

type SubscriptionServiceSuite struct {
	suite.Suite
}

func TestSubscriptionServiceSuite(t *testing.T) {
	suite.Run(t, new(SubscriptionServiceSuite))
}

func (s *SubscriptionServiceSuite) TestSubscribe() {
	cases := []struct {
		name       string
		email      string
		repo       string
		setupMocks func(*mockRepoRepository, *mockSubRepository, *mockGitHub, *mockMailer)
		wantErrIs  error
		wantAnyErr bool
		check      func(*SubscriptionServiceSuite, error)
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
			check: func(s *SubscriptionServiceSuite, err error) {
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
			name:  "subscribe succeeds, confirmation is async",
			email: "user@example.com",
			repo:  "owner/repo",
			setupMocks: func(repos *mockRepoRepository, subs *mockSubRepository, gh *mockGitHub, mailer *mockMailer) {
				existingRepo := &entity.Repository{ID: uuid.New(), Name: "owner/repo"}
				gh.On("RepoExists", mock.Anything, "owner", "repo").Return(true, nil)
				repos.On("GetByName", mock.Anything, "owner/repo").Return(existingRepo, nil)
				subs.On("Create", mock.Anything, mock.Anything).Return(nil)
				mailer.On("SendConfirmation", "user@example.com", "owner/repo", mock.Anything)
			},
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

			svc := newSvc(repos, subs, gh, mailer)
			err := svc.Subscribe(s.T().Context(), tc.email, tc.repo)
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

func (s *SubscriptionServiceSuite) TestConfirm() {
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

			svc := newSvc(&mockRepoRepository{}, subs, &mockGitHub{}, &mockMailer{})
			err := svc.Confirm(s.T().Context(), "sometoken")
			if tc.wantErrIs != nil {
				s.ErrorIs(err, tc.wantErrIs)
			} else {
				s.NoError(err)
			}
		})
	}
}

func (s *SubscriptionServiceSuite) TestUnsubscribe() {
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

			svc := newSvc(&mockRepoRepository{}, subs, &mockGitHub{}, &mockMailer{})
			err := svc.Unsubscribe(s.T().Context(), "sometoken")
			if tc.wantErrIs != nil {
				s.ErrorIs(err, tc.wantErrIs)
			} else {
				s.NoError(err)
			}
		})
	}
}

func (s *SubscriptionServiceSuite) TestGetByEmail() {
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
					{Email: "user@example.com", Repo: "owner/repo", Confirmed: true, LastSeenTag: new("v1.0.0")},
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

			svc := newSvc(&mockRepoRepository{}, subs, &mockGitHub{}, &mockMailer{})
			result, err := svc.GetByEmail(s.T().Context(), tc.email)
			switch {
			case tc.wantErrIs != nil:
				s.ErrorIs(err, tc.wantErrIs)
			case tc.wantErr:
				s.Error(err)
			default:
				s.Require().NoError(err)
				s.Len(result, tc.wantLen)
			}
		})
	}
}
