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

var ctx = context.Background()

func testRepo(name string, lastTag *string) *domain.Repository {
	return &domain.Repository{ID: uuid.New(), Name: name, LastSeenTag: lastTag}
}

func testSub(repoID uuid.UUID) *domain.Subscription {
	return &domain.Subscription{
		ID:               uuid.New(),
		RepositoryID:     repoID,
		Email:            "user@example.com",
		UnsubscribeToken: "tok",
	}
}

func testSubRepo(repoID uuid.UUID) *mockSubRepository {
	return &mockSubRepository{subs: []*domain.Subscription{testSub(repoID)}}
}

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

func TestCheckRepo_GitHubError_Skipped(t *testing.T) {
	cases := []struct {
		name string
		err  error
	}{
		{"rate limited (wrapped)", fmt.Errorf("%w, retry after 60s", domain.ErrRateLimited)},
		{"no release", domain.ErrNoRelease},
		{"unauthorized", domain.ErrUnauthorized},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			gh := &mockGitHub{err: tc.err}
			s := newScanner(&mockRepoRepository{}, &mockSubRepository{}, gh, &mockNotifier{})
			assert.NoError(t, s.checkRepo(ctx, testRepo("owner/repo", nil)))
		})
	}
}

func TestCheckRepo_TagUnchanged_NoNotification(t *testing.T) {
	gh := &mockGitHub{
		release: &domain.Release{TagName: "v1.0.0", HTMLURL: "https://github.com/owner/repo/releases/tag/v1.0.0"},
	}
	repos := &mockRepoRepository{}
	n := &mockNotifier{}
	s := newScanner(repos, &mockSubRepository{}, gh, n)

	require.NoError(t, s.checkRepo(ctx, testRepo("owner/repo", new("v1.0.0"))))
	assert.False(t, n.called)
	assert.Empty(t, repos.updatedTag)
}

func TestCheckRepo_NewRelease_SendsNotificationAndUpdatesTag(t *testing.T) {
	repo := testRepo("owner/repo", new("v1.0.0"))
	newTag := "v2.0.0"
	gh := &mockGitHub{
		release: &domain.Release{TagName: newTag, HTMLURL: "https://github.com/owner/repo/releases/tag/v2.0.0"},
	}
	repos := &mockRepoRepository{}
	n := &mockNotifier{}
	s := newScanner(repos, testSubRepo(repo.ID), gh, n)

	require.NoError(t, s.checkRepo(ctx, repo))
	assert.True(t, n.called)
	assert.Equal(t, 1, n.subCount)
	assert.Equal(t, newTag, repos.updatedTag)
	assert.Equal(t, "owner/repo", repos.updatedName)
}

func TestCheckRepo_FirstRelease_NoLastSeenTag(t *testing.T) {
	repo := testRepo("owner/repo", nil)
	gh := &mockGitHub{
		release: &domain.Release{TagName: "v1.0.0", HTMLURL: "https://github.com/owner/repo/releases/tag/v1.0.0"},
	}
	repos := &mockRepoRepository{}
	n := &mockNotifier{}
	s := newScanner(repos, testSubRepo(repo.ID), gh, n)

	require.NoError(t, s.checkRepo(ctx, repo))
	assert.True(t, n.called)
	assert.Equal(t, 1, n.subCount)
	assert.Equal(t, "v1.0.0", repos.updatedTag)
}

func TestCheckRepo_MailerError_TagNotUpdated(t *testing.T) {
	repo := testRepo("owner/repo", nil)
	gh := &mockGitHub{release: &domain.Release{TagName: "v2.0.0", HTMLURL: "..."}}
	repos := &mockRepoRepository{}
	n := &mockNotifier{err: errors.New("smtp error")}
	s := newScanner(repos, testSubRepo(repo.ID), gh, n)

	assert.Error(t, s.checkRepo(ctx, repo))
	assert.True(t, n.called)
	assert.Empty(t, repos.updatedTag)
}

func TestCheckRepo_NoSubscribers_UpdatesTagOnly(t *testing.T) {
	newTag := "v2.0.0"
	gh := &mockGitHub{release: &domain.Release{TagName: newTag, HTMLURL: "..."}}
	subs := &mockSubRepository{subs: []*domain.Subscription{}}
	repos := &mockRepoRepository{}
	n := &mockNotifier{}
	s := newScanner(repos, subs, gh, n)

	require.NoError(t, s.checkRepo(ctx, testRepo("owner/repo", nil)))
	assert.Equal(t, newTag, repos.updatedTag)
	assert.False(t, n.called)
}

func TestCheckRepo_InvalidRepo_ReturnsError(t *testing.T) {
	s := newScanner(&mockRepoRepository{}, &mockSubRepository{}, &mockGitHub{}, &mockNotifier{})
	assert.Error(t, s.checkRepo(ctx, testRepo("notavalidrepo", nil)))
}

func TestCheckRepo_SubRepoError_ReturnsError(t *testing.T) {
	gh := &mockGitHub{release: &domain.Release{TagName: "v2.0.0", HTMLURL: "..."}}
	subs := &mockSubRepository{err: errors.New("db error")}
	s := newScanner(&mockRepoRepository{}, subs, gh, &mockNotifier{})
	assert.Error(t, s.checkRepo(ctx, testRepo("owner/repo", nil)))
}

func TestCheckRepo_UpdateTagError_NoSubs_ReturnsError(t *testing.T) {
	gh := &mockGitHub{release: &domain.Release{TagName: "v2.0.0", HTMLURL: "..."}}
	subs := &mockSubRepository{subs: []*domain.Subscription{}}
	repos := &mockRepoRepository{updateErr: errors.New("db error")}
	s := newScanner(repos, subs, gh, &mockNotifier{})
	assert.Error(t, s.checkRepo(ctx, testRepo("owner/repo", nil)))
}

func TestCheckRepo_UpdateTagError_WithSubs_ReturnsError(t *testing.T) {
	repo := testRepo("owner/repo", nil)
	gh := &mockGitHub{release: &domain.Release{TagName: "v2.0.0", HTMLURL: "..."}}
	repos := &mockRepoRepository{updateErr: errors.New("db error")}
	s := newScanner(repos, testSubRepo(repo.ID), gh, &mockNotifier{})
	assert.Error(t, s.checkRepo(ctx, repo))
}
