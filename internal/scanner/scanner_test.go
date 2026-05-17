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

type mockNotifier struct {
	called   bool
	subCount int
	err      error
}

func (m *mockNotifier) Notify(
	_ context.Context,
	subs []*domain.Subscription,
	_ *domain.Repository,
	_ *domain.Release,
) error {
	m.called = true
	m.subCount = len(subs)
	return m.err
}

func newScanner(repos *mockRepoRepository, subs *mockSubRepository, gh *mockGitHub, n *mockNotifier) *Scanner {
	return NewScanner(repos, subs, gh, n, 0)
}

func TestCheckRepo_RateLimited_ReturnsNil(t *testing.T) {
	gh := &mockGitHub{err: fmt.Errorf("%w, retry after 60s", domain.ErrRateLimited)}
	s := newScanner(&mockRepoRepository{}, &mockSubRepository{}, gh, &mockNotifier{})
	repo := &domain.Repository{ID: uuid.New(), Name: "owner/repo"}

	assert.NoError(t, s.checkRepo(context.Background(), repo))
}

func TestCheckRepo_NoRelease_ReturnsNil(t *testing.T) {
	gh := &mockGitHub{err: domain.ErrNoRelease}
	s := newScanner(&mockRepoRepository{}, &mockSubRepository{}, gh, &mockNotifier{})
	repo := &domain.Repository{ID: uuid.New(), Name: "owner/repo"}

	assert.NoError(t, s.checkRepo(context.Background(), repo))
}

func TestCheckRepo_TagUnchanged_NoNotification(t *testing.T) {
	gh := &mockGitHub{
		release: &domain.Release{TagName: "v1.0.0", HTMLURL: "https://github.com/owner/repo/releases/tag/v1.0.0"},
	}
	repos := &mockRepoRepository{}
	n := &mockNotifier{}
	s := newScanner(repos, &mockSubRepository{}, gh, n)
	repo := &domain.Repository{ID: uuid.New(), Name: "owner/repo", LastSeenTag: new("v1.0.0")}

	require.NoError(t, s.checkRepo(context.Background(), repo))
	assert.False(t, n.called)
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
	n := &mockNotifier{}
	s := newScanner(repos, subs, gh, n)
	repo := &domain.Repository{ID: repoID, Name: "owner/repo", LastSeenTag: new("v1.0.0")}

	require.NoError(t, s.checkRepo(context.Background(), repo))
	assert.True(t, n.called)
	assert.Equal(t, 1, n.subCount)
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
	n := &mockNotifier{}
	s := newScanner(repos, subs, gh, n)
	repo := &domain.Repository{ID: repoID, Name: "owner/repo", LastSeenTag: nil}

	require.NoError(t, s.checkRepo(context.Background(), repo))
	assert.True(t, n.called)
	assert.Equal(t, 1, n.subCount)
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
	n := &mockNotifier{err: errors.New("smtp error")}
	s := newScanner(repos, subs, gh, n)
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
	n := &mockNotifier{}
	s := newScanner(repos, subs, gh, n)
	repo := &domain.Repository{ID: repoID, Name: "owner/repo", LastSeenTag: nil}

	require.NoError(t, s.checkRepo(context.Background(), repo))
	assert.Equal(t, newTag, repos.updatedTag)
	assert.False(t, n.called)
}
