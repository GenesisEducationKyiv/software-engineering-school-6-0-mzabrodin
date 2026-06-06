package mailer_test

import (
	"context"
	"errors"
	"io"
	"log/slog"
	"sync/atomic"
	"testing"
	"time"

	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/suite"

	"github-release-notifier/internal/adapter/mailer"
)

var testLogger = slog.New(slog.NewTextHandler(io.Discard, nil))

type mockAsyncMailer struct{ mock.Mock }

func (m *mockAsyncMailer) SendConfirmation(_ context.Context, to, repo, confirmURL string) error {
	return m.Called(to, repo, confirmURL).Error(0)
}

type ConfirmationNotifierSuite struct {
	suite.Suite
}

func TestConfirmationNotifierSuite(t *testing.T) {
	suite.Run(t, new(ConfirmationNotifierSuite))
}

func (s *ConfirmationNotifierSuite) TestSendConfirmation_FiresAsync() {
	mc := &mockAsyncMailer{}
	mc.On("SendConfirmation", mock.Anything, mock.Anything, mock.Anything).Return(nil).Once()
	defer mc.AssertExpectations(s.T())

	n := mailer.NewConfirmationNotifier(mc, testLogger)
	n.SendConfirmation(context.Background(), "a@example.com", "owner/repo", "url")
	n.Shutdown()
}

func (s *ConfirmationNotifierSuite) TestMailerErrorSwallowed() {
	mc := &mockAsyncMailer{}
	mc.On("SendConfirmation", mock.Anything, mock.Anything, mock.Anything).
		Return(errors.New("smtp error")).Once()
	defer mc.AssertExpectations(s.T())

	n := mailer.NewConfirmationNotifier(mc, testLogger)
	n.SendConfirmation(context.Background(), "a@example.com", "owner/repo", "url")
	n.Shutdown()
}

func (s *ConfirmationNotifierSuite) TestShutdownWaitsForGoroutines() {
	started := make(chan struct{})
	block := make(chan struct{})
	var goroutineDone atomic.Bool

	mc := &mockAsyncMailer{}
	mc.On("SendConfirmation", mock.Anything, mock.Anything, mock.Anything).
		Run(func(args mock.Arguments) {
			close(started)
			<-block
			goroutineDone.Store(true)
		}).Return(nil).Once()
	defer mc.AssertExpectations(s.T())

	n := mailer.NewConfirmationNotifier(mc, testLogger)
	n.SendConfirmation(context.Background(), "a@example.com", "owner/repo", "url")
	<-started

	shutdownDone := make(chan struct{})
	go func() {
		n.Shutdown()
		close(shutdownDone)
	}()

	select {
	case <-shutdownDone:
		s.T().Fatal("Shutdown returned before goroutine finished")
	case <-time.After(20 * time.Millisecond):
	}

	close(block)

	select {
	case <-shutdownDone:
	case <-time.After(time.Second):
		s.T().Fatal("Shutdown did not return after goroutine completed")
	}

	s.True(goroutineDone.Load())
}

func (s *ConfirmationNotifierSuite) TestConcurrentSends_Shutdown() {
	var count atomic.Int32
	mc := &mockAsyncMailer{}
	const sends = 10
	mc.On("SendConfirmation", mock.Anything, mock.Anything, mock.Anything).
		Run(func(args mock.Arguments) { count.Add(1) }).
		Return(nil).Times(sends)
	defer mc.AssertExpectations(s.T())

	n := mailer.NewConfirmationNotifier(mc, testLogger)
	for range sends {
		n.SendConfirmation(context.Background(), "a@example.com", "owner/repo", "url")
	}

	n.Shutdown()
	s.Equal(int32(sends), count.Load())
}
