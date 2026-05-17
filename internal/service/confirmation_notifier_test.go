package service

import (
	"errors"
	"sync/atomic"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// funcMailer calls fn on every SendConfirmation, allowing us to track calls and simulate errors.
type funcMailer struct {
	fn func() error
}

func (m *funcMailer) SendConfirmation(_, _, _ string) error {
	return m.fn()
}

func TestConfirmationNotifier_AlwaysReturnsNil(t *testing.T) {
	n := NewConfirmationNotifier(&funcMailer{fn: func() error { return nil }})
	assert.NoError(t, n.SendConfirmation("a@example.com", "owner/repo", "url"))
	n.Shutdown()
}

func TestConfirmationNotifier_MailerErrorSwallowed(t *testing.T) {
	var called atomic.Bool
	n := NewConfirmationNotifier(&funcMailer{fn: func() error {
		called.Store(true)
		return errors.New("smtp error")
	}})

	assert.NoError(t, n.SendConfirmation("a@example.com", "owner/repo", "url"))
	n.Shutdown()
	assert.True(t, called.Load(), "mailer was not attempted")
}

func TestConfirmationNotifier_ShutdownWaitsForGoroutines(t *testing.T) {
	started := make(chan struct{})
	block := make(chan struct{})
	var goroutineDone atomic.Bool

	n := NewConfirmationNotifier(&funcMailer{fn: func() error {
		close(started)
		<-block
		goroutineDone.Store(true) // set before wg.Done(), so visible after Shutdown returns
		return nil
	}})

	require.NoError(t, n.SendConfirmation("a@example.com", "owner/repo", "url"))
	<-started

	shutdownDone := make(chan struct{})
	go func() {
		n.Shutdown()
		close(shutdownDone)
	}()

	// a broken Shutdown that returns immediately would close shutdownDone near instantly and fail here
	select {
	case <-shutdownDone:
		t.Fatal("Shutdown returned before goroutine finished")
	case <-time.After(20 * time.Millisecond):
	}

	close(block)

	select {
	case <-shutdownDone:
	case <-time.After(time.Second):
		t.Fatal("Shutdown did not return after goroutine completed")
	}

	// goroutineDone.Store(true) happens before wg.Done(), which happens before wg.Wait() returns.
	// Shutdown calls wg.Wait(), so this is race-free and timing-independent.
	assert.True(t, goroutineDone.Load())
}

func TestConfirmationNotifier_ConcurrentSends_Shutdown(t *testing.T) {
	var count atomic.Int32
	n := NewConfirmationNotifier(&funcMailer{fn: func() error {
		count.Add(1)
		return nil
	}})

	const sends = 10
	for range sends {
		require.NoError(t, n.SendConfirmation("a@example.com", "owner/repo", "url"))
	}

	n.Shutdown()
	assert.Equal(t, int32(sends), count.Load())
}
