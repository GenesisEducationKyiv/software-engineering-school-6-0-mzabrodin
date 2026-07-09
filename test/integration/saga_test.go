//go:build integration

package integration

import (
	"context"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/suite"

	"github-release-notifier/internal/bootstrap"
	"github-release-notifier/internal/infrastructure/broker"
	"github-release-notifier/internal/infrastructure/db"
	"github-release-notifier/internal/infrastructure/outbox"
	sagaconsumer "github-release-notifier/internal/saga/adapter/eventconsumer"
	sagapublisher "github-release-notifier/internal/saga/adapter/eventpublisher"
	sagarepository "github-release-notifier/internal/saga/adapter/repository"
	sagadomain "github-release-notifier/internal/saga/domain"
	"github-release-notifier/internal/saga/usecase/coordinator"
	"github-release-notifier/internal/shared/events"
	subconsumer "github-release-notifier/internal/subscription/adapter/eventconsumer"
	"github-release-notifier/internal/subscription/adapter/repository"
	subdomain "github-release-notifier/internal/subscription/domain"
	"github-release-notifier/internal/subscription/usecase/compensate"
)

type SagaSuite struct {
	suite.Suite

	ctx      context.Context
	cancel   context.CancelFunc
	conn     *broker.Conn
	stopSaga func()
	stopSub  func()
}

func TestSagaSuite(t *testing.T) {
	suite.Run(t, new(SagaSuite))
}

func (s *SagaSuite) SetupSuite() {
	s.ctx, s.cancel = context.WithCancel(context.Background())

	conn, err := broker.Connect(testNATSURL, testLogger)
	s.Require().NoError(err)
	s.conn = conn

	s.Require().NoError(bootstrap.EnsureEventStreams(s.ctx, conn))

	sagaRepo := sagarepository.NewSagaRepository(testPool)
	transactor := db.NewTransactor(testPool)
	relay := outbox.NewRelay(testPool, conn, 100*time.Millisecond, 100, testLogger)
	go relay.Run(s.ctx)
	sagaPub := sagapublisher.New(relay, testLogger)
	coord := coordinator.New(sagaRepo, coordinator.NewNATSCompensator(sagaRepo, sagaPub, transactor), testLogger)
	sagaEC := sagaconsumer.New(coord, testLogger)

	s.stopSaga, err = bootstrap.StartConsumers(s.ctx, conn, []bootstrap.ConsumerSpec{
		{
			Stream:  events.StreamSubscriptions,
			Durable: "saga-subscription-pending",
			Subject: events.SubjectSubscriptionPending,
			Handler: sagaEC.HandlePending,
		},
		{
			Stream:  events.StreamSubscriptions,
			Durable: "saga-subscription-confirmed",
			Subject: events.SubjectSubscriptionConfirmed,
			Handler: sagaEC.HandleConfirmed,
		},
		{
			Stream:  events.StreamSubscriptions,
			Durable: "saga-subscription-expired",
			Subject: events.SubjectSubscriptionExpired,
			Handler: sagaEC.HandleExpired,
		},
		{
			Stream:  events.StreamNotifications,
			Durable: "saga-notification-confirmation-sent",
			Subject: events.SubjectNotificationConfirmationSent,
			Handler: sagaEC.HandleConfirmationSent,
		},
		{
			Stream:  events.StreamNotifications,
			Durable: "saga-notification-confirmation-dead",
			Subject: events.SubjectNotificationConfirmationDead,
			Handler: sagaEC.HandleConfirmationDead,
		},
	}, testLogger)
	s.Require().NoError(err)

	subs := repository.NewSubscriptionRepository(testPool)
	subEC := subconsumer.New(compensate.New(subs, testLogger), testLogger)

	s.stopSub, err = bootstrap.StartConsumers(s.ctx, conn, []bootstrap.ConsumerSpec{
		{
			Stream:  events.StreamSagas,
			Durable: "subscription-saga-compensate",
			Subject: events.SubjectSagaCompensate,
			Handler: subEC.HandleCompensate,
		},
	}, testLogger)
	s.Require().NoError(err)
}

func (s *SagaSuite) TearDownSuite() {
	s.stopSub()
	s.stopSaga()
	s.Require().NoError(s.conn.Close())
	s.cancel()
}

func (s *SagaSuite) SetupTest() {
	truncateAll(s.T())
}

func (s *SagaSuite) publish(subject string, ev any) {
	data, err := events.Marshal(ev)
	s.Require().NoError(err)
	s.Require().NoError(s.conn.Publish(s.ctx, subject, data))
}

func (s *SagaSuite) requireState(email string, want sagadomain.State) {
	s.T().Helper()
	s.Require().Eventually(func() bool {
		var state string
		err := testPool.QueryRow(s.ctx,
			`SELECT state FROM sagas WHERE type = $1 AND email = $2 AND repo_name = $3`,
			string(sagadomain.SagaTypeSubscribe), email, testRepoName).Scan(&state)

		return err == nil && state == string(want)
	}, 5*time.Second, 50*time.Millisecond, "saga for %s did not reach state %s", email, want)
}

func (s *SagaSuite) pendingSubCount(email string) int {
	var n int
	s.Require().NoError(testPool.QueryRow(s.ctx, `
		SELECT count(*) FROM subscriptions s
		JOIN repositories r ON r.id = s.repository_id
		WHERE s.email = $1 AND r.name = $2 AND s.confirmed = false
	`, email, testRepoName).Scan(&n))

	return n
}

func uniqueEmail() string {
	return "user-" + uuid.NewString() + "@example.com"
}

func (s *SagaSuite) TestHappyPathCompletesSaga() {
	email := uniqueEmail()
	sagaID := uuid.NewString()

	s.publish(events.SubjectSubscriptionPending, events.SubscriptionPending{
		SagaID:     sagaID,
		Email:      email,
		RepoName:   testRepoName,
		ConfirmURL: testBaseURL + "/api/confirm/token",
	})
	s.requireState(email, sagadomain.StatePending)

	s.publish(events.SubjectNotificationConfirmationSent, events.NotificationConfirmationSent{
		SagaID: sagaID,
		Email:  email,
	})
	s.requireState(email, sagadomain.StateConfirmationSent)

	s.publish(events.SubjectSubscriptionConfirmed, events.SubscriptionConfirmed{
		SagaID:     uuid.NewString(), // fresh sagaID — correlation falls back to (email, repo)
		Email:      email,
		RepoName:   testRepoName,
		UnsubToken: randomHex64(),
	})
	s.requireState(email, sagadomain.StateCompleted)
}

func (s *SagaSuite) TestCompensationDeletesPendingSubscription() {
	email := uniqueEmail()
	sagaID := uuid.NewString()

	repos := repository.NewGitHubRepoRepository(testPool)
	subs := repository.NewSubscriptionRepository(testPool)

	repo, err := repos.GetOrCreate(s.ctx, testRepoName)
	s.Require().NoError(err)
	sub, err := subdomain.NewSubscription(repo.ID, email)
	s.Require().NoError(err)
	s.Require().NoError(subs.Create(s.ctx, sub))
	s.Require().Equal(1, s.pendingSubCount(email))

	s.publish(events.SubjectSubscriptionPending, events.SubscriptionPending{
		SagaID:     sagaID,
		Email:      email,
		RepoName:   testRepoName,
		ConfirmURL: testBaseURL + "/api/confirm/token",
	})
	s.requireState(email, sagadomain.StatePending)

	s.publish(events.SubjectNotificationConfirmationDead, events.NotificationConfirmationDead{
		SagaID: sagaID,
		Email:  email,
		Reason: "smtp permanently failed",
	})

	s.requireState(email, sagadomain.StateCompensated)
	s.Require().Eventually(func() bool {
		return s.pendingSubCount(email) == 0
	}, 5*time.Second, 50*time.Millisecond, "pending subscription was not rolled back")
}
