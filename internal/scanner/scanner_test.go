package scanner

import (
	"context"
	"errors"
	"fmt"
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github-release-notifier/internal/domain"
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

type mockURLBuilder struct {
	base string
}

func (m *mockURLBuilder) UnsubscribeURL(token string) string {
	return m.base + "/api/unsubscribe/" + token
}

func newScanner(repos *mockRepoRepository, subs *mockSubRepository, gh *mockGitHub, mailer *mockMailer) *Scanner {
	return NewScanner(repos, subs, gh, mailer, 0, &mockURLBuilder{"http://localhost:8080"})
}

func TestCheckRepo_RateLimited_ReturnsNil(t *testing.T) {
	gh := &mockGitHub{err: fmt.Errorf("%w, retry after 60s", domain.ErrRateLimited)}
	s := newScanner(&mockRepoRepository{}, &mockSubRepository{}, gh, &mockMailer{})
	repo := &domain.Repository{ID: uuid.New(), Name: "owner/repo"}

	assert.NoError(t, s.checkRepo(context.Background(), repo))
}

func TestCheckRepo_NoRelease_ReturnsNil(t *testing.T) {
	gh := &mockGitHub{err: domain.ErrNoRelease}
	s := newScanner(&mockRepoRepository{}, &mockSubRepository{}, gh, &mockMailer{})
	repo := &domain.Repository{ID: uuid.New(), Name: "owner/repo"}

	assert.NoError(t, s.checkRepo(context.Background(), repo))
}

func TestCheckRepo_TagUnchanged_NoNotification(t *testing.T) {
	gh := &mockGitHub{
		release: &domain.Release{TagName: "v1.0.0", HTMLURL: "https://github.com/owner/repo/releases/tag/v1.0.0"},
	}
	repos := &mockRepoRepository{}
	mailer := &mockMailer{}
	s := newScanner(repos, &mockSubRepository{}, gh, mailer)
	repo := &domain.Repository{ID: uuid.New(), Name: "owner/repo", LastSeenTag: new("v1.0.0")}

	require.NoError(t, s.checkRepo(context.Background(), repo))
	assert.Empty(t, mailer.notified)
	assert.Empty(t, repos.updatedTag)
}

func TestCheckRepo_NewRelease_SendsNotificationAndUpdatesTag(t *testing.T) {
	repoID := uuid.New()
	newTag := "v2.0.0"
	gh := &mockGitHub{
		release: &domain.Release{TagName: newTag, HTMLURL: "https://github.com/owner/repo/releases/tag/v2.0.0"},
	}
	subs := &mockSubRepository{
		subs: []*domain.Subscription{
			{ID: uuid.New(), RepositoryID: repoID, Email: "user@example.com", UnsubscribeToken: "tok"},
		},
	}
	repos := &mockRepoRepository{}
	mailer := &mockMailer{}
	s := newScanner(repos, subs, gh, mailer)
	repo := &domain.Repository{ID: repoID, Name: "owner/repo", LastSeenTag: new("v1.0.0")}

	require.NoError(t, s.checkRepo(context.Background(), repo))
	assert.Len(t, mailer.notified, 1)
	assert.Equal(t, newTag, repos.updatedTag)
	assert.Equal(t, "owner/repo", repos.updatedName)
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

	require.NoError(t, s.checkRepo(context.Background(), repo))
	assert.Len(t, mailer.notified, 1)
	assert.Equal(t, "v1.0.0", repos.updatedTag)
}

func TestCheckRepo_MailerError_TagNotUpdated(t *testing.T) {
	repoID := uuid.New()
	gh := &mockGitHub{release: &domain.Release{TagName: "v2.0.0", HTMLURL: "..."}}
	subs := &mockSubRepository{
		subs: []*domain.Subscription{
			{ID: uuid.New(), RepositoryID: repoID, Email: "user@example.com", UnsubscribeToken: "tok"},
		},
	}
	repos := &mockRepoRepository{}
	mailer := &mockMailer{err: errors.New("smtp error")}
	s := newScanner(repos, subs, gh, mailer)
	repo := &domain.Repository{ID: repoID, Name: "owner/repo", LastSeenTag: nil}

	assert.Error(t, s.checkRepo(context.Background(), repo))
	assert.Empty(t, repos.updatedTag)
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

	require.NoError(t, s.checkRepo(context.Background(), repo))
	assert.Equal(t, newTag, repos.updatedTag)
	assert.Empty(t, mailer.notified)
}
