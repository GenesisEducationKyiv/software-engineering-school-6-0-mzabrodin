package unsubscribe_test

import (
	"testing"

	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/suite"

	shareddomain "github-release-notifier/internal/shared/domain"
	"github-release-notifier/internal/shared/events"
	"github-release-notifier/internal/subscription/domain"
	"github-release-notifier/internal/subscription/usecase/unsubscribe"
)

type UnsubscribeSuite struct {
	suite.Suite
}

func TestUnsubscribeSuite(t *testing.T) {
	suite.Run(t, new(UnsubscribeSuite))
}

func (s *UnsubscribeSuite) TestSuccessPublishesRemoved() {
	m := newMocks()
	defer m.assertExpectations(s.T())

	m.subs.On("Delete", mock.Anything, "sometoken").
		Return(domain.RemovedSubscription{Email: "u@example.com", Repo: "owner/repo"}, nil)
	m.tx.On("Within", mock.Anything).Return(nil)
	m.pub.On("SubscriptionRemoved", mock.Anything, mock.MatchedBy(func(ev events.SubscriptionRemoved) bool {
		return ev.Email == "u@example.com" && ev.RepoName == "owner/repo" && ev.SagaID != ""
	})).Return(nil)
	m.pub.On("Notify").Return()

	_, err := m.useCase().Execute(s.T().Context(), unsubscribe.Input{Token: "sometoken"})
	s.Require().NoError(err)
}

func (s *UnsubscribeSuite) TestNotFoundPropagates() {
	m := newMocks()
	defer m.assertExpectations(s.T())

	m.subs.On("Delete", mock.Anything, "sometoken").
		Return(domain.RemovedSubscription{}, shareddomain.ErrNotFound)
	m.tx.On("Within", mock.Anything).Return(nil)

	_, err := m.useCase().Execute(s.T().Context(), unsubscribe.Input{Token: "sometoken"})
	s.ErrorIs(err, shareddomain.ErrNotFound)

	m.pub.AssertNotCalled(s.T(), "SubscriptionRemoved", mock.Anything, mock.Anything)
	m.pub.AssertNotCalled(s.T(), "Notify")
}
