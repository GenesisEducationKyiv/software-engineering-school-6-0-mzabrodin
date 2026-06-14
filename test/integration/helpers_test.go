//go:build integration

package integration

import (
	"context"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"

	"github-release-notifier/internal/bootstrap/subscription"
	"github-release-notifier/internal/infrastructure/urlbuilder"
	"github-release-notifier/internal/shared/entity"
	"github-release-notifier/internal/shared/github"
	connectapi "github-release-notifier/internal/subscription/adapter/connectrpc"
	"github-release-notifier/internal/subscription/adapter/repository"
	"github-release-notifier/internal/subscription/usecase/confirm"
	"github-release-notifier/internal/subscription/usecase/list"
	"github-release-notifier/internal/subscription/usecase/subscribe"
	"github-release-notifier/internal/subscription/usecase/unsubscribe"
)

var testLogger = slog.New(slog.NewTextHandler(io.Discard, nil))

const (
	testAPIKey   = "test-api-key"
	testBaseURL  = "http://test.example.com"
	testEmail    = "user@example.com"
	testRepoName = "owner/repo"
)

type mockGitHub struct{ mock.Mock }

func (m *mockGitHub) RepoExists(ctx context.Context, owner, repo string) (bool, error) {
	args := m.Called(ctx, owner, repo)
	return args.Bool(0), args.Error(1)
}

func (m *mockGitHub) GetLatestRelease(ctx context.Context, owner, repo string) (*entity.Release, error) {
	args := m.Called(ctx, owner, repo)
	rel, _ := args.Get(0).(*entity.Release)
	return rel, args.Error(1)
}

type mockConfirmationNotifier struct{ mock.Mock }

func (m *mockConfirmationNotifier) SendConfirmation(_ context.Context, email, repo, url string) {
	m.Called(email, repo, url)
}

type mockReleaseNotifier struct{ mock.Mock }

func (m *mockReleaseNotifier) Notify(
	_ context.Context,
	subs []*entity.Subscription,
	repo *entity.Repository,
	release *entity.Release,
) error {
	return m.Called(subs, repo, release).Error(0)
}

type testUseCases struct {
	subscribe   *subscribe.UseCase
	confirm     *confirm.UseCase
	unsubscribe *unsubscribe.UseCase
	list        *list.UseCase
}

func newTestUseCases(repoExists bool) testUseCases {
	gh := &mockGitHub{}
	gh.On("RepoExists", mock.Anything, mock.Anything, mock.Anything).Return(repoExists, nil).Maybe()
	gh.On("GetLatestRelease", mock.Anything, mock.Anything, mock.Anything).
		Return(nil, github.ErrNoRelease).Maybe()

	notifier := &mockConfirmationNotifier{}
	notifier.On("SendConfirmation", mock.Anything, mock.Anything, mock.Anything).Return(nil).Maybe()

	releaseNotifier := &mockReleaseNotifier{}
	releaseNotifier.On("Notify", mock.Anything, mock.Anything, mock.Anything).Return(nil).Maybe()

	repos := repository.NewGitHubRepoRepository(testPool)
	subs := repository.NewSubscriptionRepository(testPool)
	urls := urlbuilder.New(testBaseURL)

	return testUseCases{
		subscribe:   subscribe.New(repos, subs, gh, notifier, urls, testLogger),
		confirm:     confirm.New(subs, gh, releaseNotifier, testLogger),
		unsubscribe: unsubscribe.New(subs, testLogger),
		list:        list.New(subs),
	}
}

func newTestServer(t *testing.T, repoExists bool) *httptest.Server {
	t.Helper()

	uc := newTestUseCases(repoExists)
	svc := connectapi.NewService(uc.subscribe, uc.confirm, uc.unsubscribe, uc.list, testLogger)

	handler, err := subscription.NewHandler(svc, testAPIKey, testLogger)
	require.NoError(t, err)

	srv := httptest.NewUnstartedServer(handler)
	srv.Config.Protocols = new(http.Protocols)
	srv.Config.Protocols.SetHTTP1(true)
	srv.Config.Protocols.SetUnencryptedHTTP2(true)
	srv.Start()
	t.Cleanup(srv.Close)

	return srv
}

func truncateAll(t *testing.T) {
	t.Helper()
	_, err := testPool.Exec(t.Context(), "TRUNCATE subscriptions, repositories CASCADE")
	require.NoError(t, err)
}

func randomHex64() string {
	return strings.Repeat("ab", 32)
}
