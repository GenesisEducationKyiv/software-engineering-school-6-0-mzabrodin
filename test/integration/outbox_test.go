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
	"github-release-notifier/internal/infrastructure/outbox"
	"github-release-notifier/internal/shared/events"
)

type OutboxSuite struct {
	suite.Suite

	conn     *broker.Conn
	relay    *outbox.Relay
	received chan []byte
	stop     func()
}

func TestOutboxSuite(t *testing.T) {
	suite.Run(t, new(OutboxSuite))
}

func (s *OutboxSuite) SetupTest() {
	ctx := s.T().Context()

	// The outbox_messages table is created by the embedded outbox migration in main_test.go
	// here we only reset it between tests.
	_, err := testPool.Exec(ctx, "TRUNCATE outbox_messages")
	s.Require().NoError(err)

	conn, err := broker.Connect(testNATSURL, testLogger)
	s.Require().NoError(err)
	s.conn = conn

	s.Require().NoError(bootstrap.EnsureEventStreams(ctx, conn))

	s.received = make(chan []byte, 16)
	stop, err := conn.Consume(ctx, events.StreamReleases, "test-outbox-release",
		events.SubjectReleaseDetected, func(_ context.Context, data []byte) error {
			s.received <- data

			return nil
		})
	s.Require().NoError(err)
	s.stop = stop

	s.relay = outbox.NewRelay(testPool, conn, 100*time.Millisecond, 10, testLogger)
	go s.relay.Run(ctx)
}

func (s *OutboxSuite) TearDownTest() {
	s.stop()
	s.Require().NoError(s.conn.Close())

	_, err := testPool.Exec(s.T().Context(), "TRUNCATE outbox_messages")
	s.Require().NoError(err)
}

func (s *OutboxSuite) outboxCount() int {
	var n int
	s.Require().NoError(testPool.QueryRow(s.T().Context(),
		"SELECT count(*) FROM outbox_messages").Scan(&n))

	return n
}

func (s *OutboxSuite) detected() events.ReleaseDetected {
	return events.ReleaseDetected{
		SagaID:     uuid.NewString(),
		RepoName:   "golang/go",
		Tag:        "v1.26.0",
		ReleaseURL: "https://github.com/golang/go/releases/tag/v1.26.0",
	}
}

func (s *OutboxSuite) TestRelayPublishesCommittedRowsAndClearsThem() {
	ctx := s.T().Context()

	ev := s.detected()
	payload, err := events.Marshal(ev)
	s.Require().NoError(err)

	tx, err := testPool.Begin(ctx)
	s.Require().NoError(err)
	s.Require().NoError(outbox.Enqueue(ctx, tx, events.SubjectReleaseDetected, payload))
	s.Require().NoError(tx.Commit(ctx))

	s.relay.Notify()

	select {
	case data := <-s.received:
		got, decErr := events.Unmarshal[events.ReleaseDetected](data)
		s.Require().NoError(decErr)
		s.Equal(ev, got)
	case <-time.After(5 * time.Second):
		s.Fail("committed event was not published")
	}

	s.Require().Eventually(func() bool { return s.outboxCount() == 0 },
		5*time.Second, 50*time.Millisecond, "published row was not deleted")
}

func (s *OutboxSuite) TestRolledBackEnqueueIsNotPublished() {
	ctx := s.T().Context()

	payload, err := events.Marshal(s.detected())
	s.Require().NoError(err)

	tx, err := testPool.Begin(ctx)
	s.Require().NoError(err)
	s.Require().NoError(outbox.Enqueue(ctx, tx, events.SubjectReleaseDetected, payload))
	s.Require().NoError(tx.Rollback(ctx))

	s.relay.Notify()

	select {
	case <-s.received:
		s.Fail("rolled-back event must not be published")
	case <-time.After(500 * time.Millisecond):
	}

	s.Equal(0, s.outboxCount())
}
