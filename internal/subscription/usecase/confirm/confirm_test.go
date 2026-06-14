package confirm_test

import (
	"testing"
	"time"

	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/suite"

	"github-release-notifier/internal/shared/entity"
	"github-release-notifier/internal/shared/github"
	"github-release-notifier/internal/subscription/usecase/confirm"
)

type ConfirmSuite struct {
	suite.Suite
}

func TestConfirmSuite(t *testing.T) {
	suite.Run(t, new(ConfirmSuite))
}

func (s *ConfirmSuite) requireClosed(ch <-chan struct{}) {
	select {
	case <-ch:
	case <-time.After(2 * time.Second):
		s.FailNow("timed out waiting for async welcome send")
	}
}

func (s *ConfirmSuite) TestReturnsErrorWhenNotFound() {
	subs := &mockSubRepository{}
	subs.On("Confirm", mock.Anything, "token").Return(nil, "", entity.ErrNotFound)
	defer subs.AssertExpectations(s.T())

	uc := confirm.New(subs, &mockGitHub{}, &mockNotifier{}, testLogger)
	_, err := uc.Execute(s.T().Context(), confirm.Input{Token: "token"})
	s.ErrorIs(err, entity.ErrNotFound)
}

func (s *ConfirmSuite) TestIdempotentReconfirmSkipsWelcome() {
	subs := &mockSubRepository{}
	subs.On("Confirm", mock.Anything, "token").Return(nil, "", nil)
	defer subs.AssertExpectations(s.T())

	gh := &mockGitHub{}
	notifier := &mockNotifier{}

	uc := confirm.New(subs, gh, notifier, testLogger)
	_, err := uc.Execute(s.T().Context(), confirm.Input{Token: "token"})
	s.Require().NoError(err)

	gh.AssertNotCalled(s.T(), "GetLatestRelease", mock.Anything, mock.Anything, mock.Anything)
	notifier.AssertNotCalled(s.T(), "Notify", mock.Anything, mock.Anything, mock.Anything, mock.Anything)
}

func (s *ConfirmSuite) TestSendsWelcomeReleaseOnFreshConfirm() {
	sub := &entity.Subscription{Email: "u@example.com", UnsubscribeToken: "tok"}
	subs := &mockSubRepository{}
	subs.On("Confirm", mock.Anything, "token").Return(sub, "owner/repo", nil)

	release := &entity.Release{TagName: "v1.0.0", HTMLURL: "https://github.com/owner/repo/releases/tag/v1.0.0"}
	gh := &mockGitHub{}
	gh.On("GetLatestRelease", mock.Anything, "owner", "repo").Return(release, nil)

	done := make(chan struct{})
	notifier := &mockNotifier{}
	notifier.On("Notify", mock.Anything,
		mock.MatchedBy(func(subs []*entity.Subscription) bool {
			return len(subs) == 1 && subs[0].Email == "u@example.com"
		}),
		mock.Anything, release,
	).Run(func(mock.Arguments) { close(done) }).Return(nil)

	uc := confirm.New(subs, gh, notifier, testLogger)
	_, err := uc.Execute(s.T().Context(), confirm.Input{Token: "token"})
	s.Require().NoError(err)

	s.requireClosed(done)
	subs.AssertExpectations(s.T())
	gh.AssertExpectations(s.T())
	notifier.AssertExpectations(s.T())
}

func (s *ConfirmSuite) TestSkipsWelcomeWhenNoRelease() {
	sub := &entity.Subscription{Email: "u@example.com", UnsubscribeToken: "tok"}
	subs := &mockSubRepository{}
	subs.On("Confirm", mock.Anything, "token").Return(sub, "owner/repo", nil)

	done := make(chan struct{})
	gh := &mockGitHub{}
	gh.On("GetLatestRelease", mock.Anything, "owner", "repo").
		Run(func(mock.Arguments) { close(done) }).
		Return(nil, github.ErrNoRelease)

	notifier := &mockNotifier{}

	uc := confirm.New(subs, gh, notifier, testLogger)
	_, err := uc.Execute(s.T().Context(), confirm.Input{Token: "token"})
	s.Require().NoError(err)

	s.requireClosed(done)
	gh.AssertExpectations(s.T())
	notifier.AssertNotCalled(s.T(), "Notify", mock.Anything, mock.Anything, mock.Anything, mock.Anything)
}
