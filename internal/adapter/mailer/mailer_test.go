package mailer

import (
	"context"
	"errors"
	"io"
	"log/slog"
	"sync"
	"testing"
	"time"

	"github.com/prometheus/client_golang/prometheus/testutil"
	"github.com/stretchr/testify/suite"
	"github.com/wneessen/go-mail"

	"github-release-notifier/internal/entity"
	"github-release-notifier/internal/infrastructure/metrics"
)

var testLogger = slog.New(slog.NewTextHandler(io.Discard, nil))

// fakeSender stands in for the SMTP layer so dispatcher/batch logic is testable
// without a real server. errFn decides each message's outcome by recipient.
type fakeSender struct {
	mu    sync.Mutex
	sent  [][]*mail.Msg
	errFn func(msg *mail.Msg) error
}

func (f *fakeSender) sendBatch(_ context.Context, msgs []*mail.Msg) []error {
	f.mu.Lock()
	f.sent = append(f.sent, msgs)
	f.mu.Unlock()

	errs := make([]error, len(msgs))
	if f.errFn != nil {
		for i, msg := range msgs {
			errs[i] = f.errFn(msg)
		}
	}

	return errs
}

func (f *fakeSender) batches() int {
	f.mu.Lock()
	defer f.mu.Unlock()
	return len(f.sent)
}

type MailerSuite struct {
	suite.Suite
}

func TestMailerSuite(t *testing.T) {
	suite.Run(t, new(MailerSuite))
}

func notification(to string) entity.ReleaseNotification {
	return entity.ReleaseNotification{
		To:             to,
		Repo:           "owner/repo",
		Tag:            "v1.0.0",
		ReleaseURL:     "https://github.com/owner/repo/releases/tag/v1.0.0",
		UnsubscribeURL: "https://example.com/unsubscribe/" + to,
	}
}

func (s *MailerSuite) TestConfirmationTemplate() {
	body, err := renderTemplate(confirmationTemplate, map[string]string{
		"Repo":       "golang/go",
		"ConfirmURL": "http://localhost/confirm/abc",
	})
	s.Require().NoError(err)
	s.Contains(body, "golang/go")
	s.Contains(body, "http://localhost/confirm/abc")
	s.Contains(body, "<html>")
}

func (s *MailerSuite) TestReleaseTemplate() {
	body, err := renderTemplate(releaseTemplate, map[string]string{
		"Repo":           "owner/repo",
		"Tag":            "v2.0.0",
		"ReleaseURL":     "http://github.com/release",
		"UnsubscribeURL": "http://example.com/unsub",
	})
	s.Require().NoError(err)
	s.Contains(body, "owner/repo")
	s.Contains(body, "v2.0.0")
	s.Contains(body, "http://github.com/release")
	s.Contains(body, "http://example.com/unsub")
	s.Contains(body, "<html>")
}

func (s *MailerSuite) TestSendReleaseNotifications_Empty() {
	fake := &fakeSender{}
	m := newMailer(fake, "from@example.com", testLogger)
	defer m.Shutdown(context.Background())

	result := m.SendReleaseNotifications(context.Background(), nil)

	s.Equal(0, result.Sent)
	s.Empty(result.Failed)
	s.Equal(0, fake.batches())
}

func (s *MailerSuite) TestSendReleaseNotifications_AllSuccess() {
	fake := &fakeSender{}
	m := newMailer(fake, "from@example.com", testLogger)
	defer m.Shutdown(context.Background())

	before := testutil.ToFloat64(metrics.EmailSendsTotal.WithLabelValues(kindNotification, "success"))

	result := m.SendReleaseNotifications(context.Background(), []entity.ReleaseNotification{
		notification("a@example.com"),
		notification("b@example.com"),
	})

	s.Equal(2, result.Sent)
	s.Empty(result.Failed)
	s.InDelta(
		2,
		testutil.ToFloat64(metrics.EmailSendsTotal.WithLabelValues(kindNotification, "success"))-before,
		0.0001,
	)
}

func (s *MailerSuite) TestSendReleaseNotifications_PartialFailure() {
	fake := &fakeSender{errFn: func(msg *mail.Msg) error {
		if recipient(msg) == "b@example.com" {
			return errors.New("smtp error")
		}
		return nil
	}}
	m := newMailer(fake, "from@example.com", testLogger)
	defer m.Shutdown(context.Background())

	beforeOK := testutil.ToFloat64(metrics.EmailSendsTotal.WithLabelValues(kindNotification, "success"))
	beforeErr := testutil.ToFloat64(metrics.EmailSendsTotal.WithLabelValues(kindNotification, "error"))

	result := m.SendReleaseNotifications(context.Background(), []entity.ReleaseNotification{
		notification("a@example.com"),
		notification("b@example.com"),
		notification("c@example.com"),
	})

	s.Equal(2, result.Sent)
	s.Equal([]string{"b@example.com"}, result.Failed)
	s.InDelta(
		2,
		testutil.ToFloat64(metrics.EmailSendsTotal.WithLabelValues(kindNotification, "success"))-beforeOK,
		0.0001,
	)
	s.InDelta(
		1,
		testutil.ToFloat64(metrics.EmailSendsTotal.WithLabelValues(kindNotification, "error"))-beforeErr,
		0.0001,
	)
}

func (s *MailerSuite) TestSendConfirmation_Enqueued() {
	fake := &fakeSender{}
	m := newMailer(fake, "from@example.com", testLogger)
	defer m.Shutdown(context.Background())

	before := testutil.ToFloat64(metrics.EmailSendsTotal.WithLabelValues(kindConfirmation, "success"))
	m.SendConfirmation(context.Background(), "a@example.com", "owner/repo", "http://confirm")

	s.Require().Eventually(func() bool {
		return testutil.ToFloat64(metrics.EmailSendsTotal.WithLabelValues(kindConfirmation, "success"))-before >= 1
	}, time.Second, time.Millisecond)
	s.Equal(1, fake.batches())
}

func (s *MailerSuite) TestShutdown_DrainsQueuedConfirmations() {
	fake := &fakeSender{}
	m := newMailer(fake, "from@example.com", testLogger)

	const n = 5
	for range n {
		m.SendConfirmation(context.Background(), "a@example.com", "owner/repo", "http://confirm")
	}

	m.Shutdown(context.Background())

	s.Equal(n, fake.batches())
}

func (s *MailerSuite) TestNewMailer_Constructs() {
	m, err := NewMailer("localhost", 587, "user", "pass", "from@example.com", testLogger)
	s.Require().NoError(err)
	m.Shutdown(context.Background())
}
