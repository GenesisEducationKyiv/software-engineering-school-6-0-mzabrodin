package scanner

import (
	"context"
	"errors"
	"fmt"
	"testing"

	"github-release-notifier/internal/domain"

	"github.com/google/uuid"
)

type mockRepoRepository struct {
	repos       []*domain.Repository
	updatedName string
	updatedTag  string
	updateErr   error
}

func (m *mockRepoRepository) GetAllWithSubscriptions(_ context.Context) ([]*domain.Repository, error) {
	return m.repos, nil
}

func (m *mockRepoRepository) UpdateLastSeenTag(_ context.Context, name, tag string) error {
	m.updatedName = name
	m.updatedTag = tag
	return m.updateErr
}

type mockSubRepository struct {
	subs []*domain.Subscription
	err  error
}

func (m *mockSubRepository) GetConfirmedByRepoID(_ context.Context, _ uuid.UUID) ([]*domain.Subscription, error) {
	return m.subs, m.err
}

type mockGitHub struct {
	release *domain.Release
	err     error
}

func (m *mockGitHub) GetLatestRelease(_ context.Context, _, _ string) (*domain.Release, error) {
	return m.release, m.err
}

type mockMailer struct {
	notified []domain.ReleaseNotification
	err      error
}

func (m *mockMailer) SendReleaseNotifications(notifications []domain.ReleaseNotification) error {
	m.notified = notifications
	return m.err
}

func newScanner(repos *mockRepoRepository, subs *mockSubRepository, gh *mockGitHub, mailer *mockMailer) *Scanner {
	return NewScanner(repos, subs, gh, mailer, 0, "http://localhost:8080")
}

func TestSplitRepo_Valid(t *testing.T) {
	owner, name, err := splitRepo("golang/go")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if owner != "golang" || name != "go" {
		t.Errorf("got (%q, %q), want (\"golang\", \"go\")", owner, name)
	}
}

func TestSplitRepo_Invalid(t *testing.T) {
	cases := []string{"", "noslash", "/repo", "owner/"}
	for _, tc := range cases {
		_, _, err := splitRepo(tc)
		if err == nil {
			t.Errorf("splitRepo(%q): expected error, got nil", tc)
		}
	}
}

func TestCheckRepo_RateLimited_ReturnsNil(t *testing.T) {
	gh := &mockGitHub{err: fmt.Errorf("%w, retry after 60s", domain.ErrRateLimited)}
	s := newScanner(&mockRepoRepository{}, &mockSubRepository{}, gh, &mockMailer{})
	repo := &domain.Repository{ID: uuid.New(), Name: "owner/repo"}

	if err := s.checkRepo(context.Background(), repo); err != nil {
		t.Errorf("expected nil (skip), got %v", err)
	}
}

func TestCheckRepo_NoRelease_ReturnsNil(t *testing.T) {
	gh := &mockGitHub{err: domain.ErrNoRelease}
	s := newScanner(&mockRepoRepository{}, &mockSubRepository{}, gh, &mockMailer{})
	repo := &domain.Repository{ID: uuid.New(), Name: "owner/repo"}

	if err := s.checkRepo(context.Background(), repo); err != nil {
		t.Errorf("expected nil, got %v", err)
	}
}

func TestCheckRepo_TagUnchanged_NoNotification(t *testing.T) {
	gh := &mockGitHub{
		release: &domain.Release{TagName: "v1.0.0", HTMLURL: "https://github.com/owner/repo/releases/tag/v1.0.0"},
	}
	repos := &mockRepoRepository{}
	mailer := &mockMailer{}
	s := newScanner(repos, &mockSubRepository{}, gh, mailer)
	repo := &domain.Repository{ID: uuid.New(), Name: "owner/repo", LastSeenTag: new("v1.0.0")}

	if err := s.checkRepo(context.Background(), repo); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(mailer.notified) != 0 {
		t.Error("expected no notifications for unchanged tag")
	}

	if repos.updatedTag != "" {
		t.Error("expected no DB update for unchanged tag")
	}
}

func TestCheckRepo_NewRelease_SendsNotificationAndUpdatesTag(t *testing.T) {
	subID := uuid.New()
	repoID := uuid.New()
	newTag := "v2.0.0"

	gh := &mockGitHub{
		release: &domain.Release{TagName: newTag, HTMLURL: "https://github.com/owner/repo/releases/tag/v2.0.0"},
	}
	subs := &mockSubRepository{
		subs: []*domain.Subscription{
			{ID: subID, RepositoryID: repoID, Email: "user@example.com", UnsubscribeToken: "tok"},
		},
	}
	repos := &mockRepoRepository{}
	mailer := &mockMailer{}
	s := newScanner(repos, subs, gh, mailer)
	repo := &domain.Repository{ID: repoID, Name: "owner/repo", LastSeenTag: new("v1.0.0")}

	if err := s.checkRepo(context.Background(), repo); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(mailer.notified) != 1 {
		t.Errorf("expected 1 notification, got %d", len(mailer.notified))
	}

	if repos.updatedTag != newTag {
		t.Errorf("expected updated tag %q, got %q", newTag, repos.updatedTag)
	}

	if repos.updatedName != "owner/repo" {
		t.Errorf("expected updated name %q, got %q", "owner/repo", repos.updatedName)
	}
}

func TestCheckRepo_FirstRelease_NoLastSeenTag(t *testing.T) {
	repoID := uuid.New()
	gh := &mockGitHub{
		release: &domain.Release{TagName: "v1.0.0", HTMLURL: "https://github.com/owner/repo/releases/tag/v1.0.0"},
	}
	subs := &mockSubRepository{
		subs: []*domain.Subscription{
			{ID: uuid.New(), RepositoryID: repoID, Email: "user@example.com", UnsubscribeToken: "tok"},
		},
	}
	repos := &mockRepoRepository{}
	mailer := &mockMailer{}
	s := newScanner(repos, subs, gh, mailer)
	repo := &domain.Repository{ID: repoID, Name: "owner/repo", LastSeenTag: nil}

	if err := s.checkRepo(context.Background(), repo); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(mailer.notified) != 1 {
		t.Errorf("expected 1 notification, got %d", len(mailer.notified))
	}
}

func TestCheckRepo_MailerError_TagStillUpdated(t *testing.T) {
	repoID := uuid.New()
	newTag := "v2.0.0"
	gh := &mockGitHub{release: &domain.Release{TagName: newTag, HTMLURL: "..."}}
	subs := &mockSubRepository{
		subs: []*domain.Subscription{
			{ID: uuid.New(), RepositoryID: repoID, Email: "user@example.com", UnsubscribeToken: "tok"},
		},
	}
	repos := &mockRepoRepository{}
	mailer := &mockMailer{err: errors.New("smtp error")}
	s := newScanner(repos, subs, gh, mailer)
	repo := &domain.Repository{ID: repoID, Name: "owner/repo", LastSeenTag: nil}

	err := s.checkRepo(context.Background(), repo)
	if err != nil {
		t.Errorf("expected nil (mailer error only logged), got %v", err)
	}

	if repos.updatedTag != newTag {
		t.Errorf("expected tag %q to be updated despite mailer error, got %q", newTag, repos.updatedTag)
	}
}

func TestCheckRepo_NoSubscribers_UpdatesTagOnly(t *testing.T) {
	repoID := uuid.New()
	newTag := "v2.0.0"
	gh := &mockGitHub{release: &domain.Release{TagName: newTag, HTMLURL: "..."}}
	subs := &mockSubRepository{subs: []*domain.Subscription{}}
	repos := &mockRepoRepository{}
	mailer := &mockMailer{}
	s := newScanner(repos, subs, gh, mailer)
	repo := &domain.Repository{ID: repoID, Name: "owner/repo", LastSeenTag: nil}

	if err := s.checkRepo(context.Background(), repo); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if repos.updatedTag != newTag {
		t.Errorf("expected tag %q to be updated, got %q", newTag, repos.updatedTag)
	}

	if len(mailer.notified) != 0 {
		t.Error("expected no notifications for empty subscribers")
	}
}
