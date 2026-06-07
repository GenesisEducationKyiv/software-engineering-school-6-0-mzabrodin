package scheduler_test

import (
	"context"
	"io"
	"log/slog"
	"testing"
	"time"

	"github.com/stretchr/testify/suite"

	"github-release-notifier/internal/infrastructure/scheduler"
)

var testLogger = slog.New(slog.NewTextHandler(io.Discard, nil))

type mockScanner struct {
	runs chan struct{}
}

func (m *mockScanner) Run(_ context.Context) error {
	select {
	case m.runs <- struct{}{}:
	default:
	}

	return nil
}

type SchedulerSuite struct {
	suite.Suite
}

func TestSchedulerSuite(t *testing.T) {
	suite.Run(t, new(SchedulerSuite))
}

func (s *SchedulerSuite) TestRunsAtStartupAndPerTick() {
	scanner := &mockScanner{runs: make(chan struct{}, 10)}
	sched := scheduler.New(scanner, 20*time.Millisecond, testLogger)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	go sched.Start(ctx)

	s.Require().
		Eventually(func() bool { return len(scanner.runs) >= 1 }, time.Second, time.Millisecond, "no startup run")
	s.Require().Eventually(func() bool { return len(scanner.runs) >= 2 }, time.Second, time.Millisecond, "no tick run")
}

func (s *SchedulerSuite) TestStopsOnContextCancel() {
	scanner := &mockScanner{runs: make(chan struct{}, 10)}
	sched := scheduler.New(scanner, time.Millisecond, testLogger)

	ctx, cancel := context.WithCancel(context.Background())

	done := make(chan struct{})
	go func() {
		sched.Start(ctx)
		close(done)
	}()

	cancel()

	select {
	case <-done:
	case <-time.After(time.Second):
		s.Fail("Start did not return after context cancel")
	}
}
