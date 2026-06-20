package confirm_test

import (
	"testing"

	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/suite"

	shareddomain "github-release-notifier/internal/shared/domain"
	"github-release-notifier/internal/shared/events"
	"github-release-notifier/internal/subscription/adapter/confirmtoken"
	"github-release-notifier/internal/subscription/domain"
	"github-release-notifier/internal/subscription/usecase/confirm"
)

type ConfirmSuite struct {
	suite.Suite
}

func TestConfirmSuite(t *testing.T) {
	suite.Run(t, new(ConfirmSuite))
}

func (s *ConfirmSuite) TestInvalidTokenReturnsError() {
	m := newMocks()
	defer m.assertExpectations(s.T())

	m.tokens.On("Verify", "bad").Return("", "", confirmtoken.ErrInvalid)

	_, err := m.useCase().Execute(s.T().Context(), confirm.Input{Token: "bad"})
	s.ErrorIs(err, confirmtoken.ErrInvalid)
}

func (s *ConfirmSuite) TestExpiredTokenReturnsError() {
	m := newMocks()
	defer m.assertExpectations(s.T())

	m.tokens.On("Verify", "expired").Return("", "", confirmtoken.ErrExpired)

	_, err := m.useCase().Execute(s.T().Context(), confirm.Input{Token: "expired"})
	s.ErrorIs(err, confirmtoken.ErrExpired)
}

func (s *ConfirmSuite) TestFreshConfirmPublishesConfirmed() {
	m := newMocks()
	defer m.assertExpectations(s.T())

	m.tokens.On("Verify", "token").Return("u@example.com", "owner/repo", nil)
	m.tx.On("Within", mock.Anything).Return(nil)
	m.subs.On("Confirm", mock.Anything, "u@example.com", "owner/repo").
		Return(domain.ConfirmResult{Confirmed: true, UnsubToken: "unsub-tok"}, nil)
	m.pub.On("SubscriptionConfirmed", mock.Anything, mock.MatchedBy(func(ev events.SubscriptionConfirmed) bool {
		return ev.Email == "u@example.com" &&
			ev.RepoName == "owner/repo" &&
			ev.UnsubToken == "unsub-tok" &&
			ev.SagaID != ""
	})).Return(nil)
	m.pub.On("Notify").Return()

	_, err := m.useCase().Execute(s.T().Context(), confirm.Input{Token: "token"})
	s.Require().NoError(err)
}

func (s *ConfirmSuite) TestIdempotentReconfirmPublishesNothing() {
	m := newMocks()
	defer m.assertExpectations(s.T())

	m.tokens.On("Verify", "token").Return("u@example.com", "owner/repo", nil)
	m.tx.On("Within", mock.Anything).Return(nil)
	m.subs.On("Confirm", mock.Anything, "u@example.com", "owner/repo").
		Return(domain.ConfirmResult{Confirmed: false}, nil)

	_, err := m.useCase().Execute(s.T().Context(), confirm.Input{Token: "token"})
	s.Require().NoError(err)

	m.pub.AssertNotCalled(s.T(), "SubscriptionConfirmed", mock.Anything, mock.Anything)
	m.pub.AssertNotCalled(s.T(), "Notify")
}

func (s *ConfirmSuite) TestConfirmRepositoryErrorPropagates() {
	m := newMocks()
	defer m.assertExpectations(s.T())

	m.tokens.On("Verify", "token").Return("u@example.com", "owner/repo", nil)
	m.tx.On("Within", mock.Anything).Return(nil)
	m.subs.On("Confirm", mock.Anything, "u@example.com", "owner/repo").
		Return(domain.ConfirmResult{}, shareddomain.ErrNotFound)

	_, err := m.useCase().Execute(s.T().Context(), confirm.Input{Token: "token"})
	s.ErrorIs(err, shareddomain.ErrNotFound)
}
