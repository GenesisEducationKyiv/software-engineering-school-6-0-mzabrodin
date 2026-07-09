package cleanup_test

import (
	"testing"
	"time"

	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/suite"

	"github-release-notifier/internal/shared/events"
	"github-release-notifier/internal/subscription/domain"
)

type CleanupSuite struct {
	suite.Suite
}

func TestCleanupSuite(t *testing.T) {
	suite.Run(t, new(CleanupSuite))
}

func (s *CleanupSuite) TestPublishesExpiredAndNotifies() {
	m := newMocks()
	defer m.assertExpectations(s.T())

	m.subs.On("DeleteExpiredPending", mock.Anything, mock.Anything).Return([]domain.ExpiredSubscription{
		{Email: "a@example.com", Repo: "owner/one"},
		{Email: "b@example.com", Repo: "owner/two"},
	}, nil)
	m.tx.On("Within", mock.Anything).Return(nil)
	m.pub.On("SubscriptionExpired", mock.Anything, mock.MatchedBy(func(ev events.SubscriptionExpired) bool {
		return ev.SagaID != "" && ev.Email != "" && ev.RepoName != ""
	})).Return(nil).Twice()
	m.pub.On("Notify").Return()

	s.Require().NoError(m.useCase(24 * time.Hour).Run(s.T().Context()))
}

func (s *CleanupSuite) TestNoExpiredSkipsNotify() {
	m := newMocks()
	defer m.assertExpectations(s.T())

	m.subs.On("DeleteExpiredPending", mock.Anything, mock.Anything).
		Return([]domain.ExpiredSubscription{}, nil)
	m.tx.On("Within", mock.Anything).Return(nil)

	s.Require().NoError(m.useCase(24 * time.Hour).Run(s.T().Context()))

	m.pub.AssertNotCalled(s.T(), "SubscriptionExpired", mock.Anything, mock.Anything)
	m.pub.AssertNotCalled(s.T(), "Notify")
}

func (s *CleanupSuite) TestUsesMaxAgeForCutoff() {
	m := newMocks()
	defer m.assertExpectations(s.T())

	m.subs.On("DeleteExpiredPending", mock.Anything, mock.MatchedBy(func(cutoff time.Time) bool {
		return time.Since(cutoff) > 59*time.Minute && time.Since(cutoff) < 61*time.Minute
	})).Return([]domain.ExpiredSubscription{}, nil)
	m.tx.On("Within", mock.Anything).Return(nil)

	s.Require().NoError(m.useCase(time.Hour).Run(s.T().Context()))
}
