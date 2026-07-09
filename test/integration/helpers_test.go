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
	"time"

	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"

	"github-release-notifier/internal/bootstrap/subscription"
	"github-release-notifier/internal/infrastructure/db"
	"github-release-notifier/internal/infrastructure/urlbuilder"
	"github-release-notifier/internal/shared/domain"
	"github-release-notifier/internal/subscription/adapter/confirmtoken"
	connectapi "github-release-notifier/internal/subscription/adapter/connectrpc"
	"github-release-notifier/internal/subscription/adapter/eventpublisher"
	"github-release-notifier/internal/subscription/adapter/repository"
	"github-release-notifier/internal/subscription/usecase/confirm"
	"github-release-notifier/internal/subscription/usecase/list"
	"github-release-notifier/internal/subscription/usecase/subscribe"
	"github-release-notifier/internal/subscription/usecase/unsubscribe"
)

var testLogger = slog.New(slog.NewTextHandler(io.Discard, nil))

const (
	testAPIKey     = "test-api-key"
	testBaseURL    = "http://test.example.com"
	testEmail      = "user@example.com"
	testRepoName   = "owner/repo"
	testJWTSecret  = "integration-test-secret"
	testConfirmTTL = time.Hour
)

var testTokenizer = confirmtoken.New(testJWTSecret, testConfirmTTL)

type mockGitHub struct{ mock.Mock }

func (m *mockGitHub) RepoExists(ctx context.Context, owner, repo string) (bool, error) {
	args := m.Called(ctx, owner, repo)
	return args.Bool(0), args.Error(1)
}

func (m *mockGitHub) GetLatestRelease(ctx context.Context, owner, repo string) (*domain.Release, error) {
	args := m.Called(ctx, owner, repo)
	rel, _ := args.Get(0).(*domain.Release)
	return rel, args.Error(1)
}

type mockRelay struct{ mock.Mock }

func (m *mockRelay) Notify() { m.Called() }

type testUseCases struct {
	subscribe   *subscribe.UseCase
	confirm     *confirm.UseCase
	unsubscribe *unsubscribe.UseCase
	list        *list.UseCase
}

func newTestUseCases(repoExists bool) testUseCases {
	gh := &mockGitHub{}
	gh.On("RepoExists", mock.Anything, mock.Anything, mock.Anything).Return(repoExists, nil).Maybe()

	repos := repository.NewGitHubRepoRepository(testPool)
	subs := repository.NewSubscriptionRepository(testPool)
	urls := urlbuilder.New(testBaseURL)
	tx := db.NewTransactor(testPool)
	relay := &mockRelay{}
	relay.On("Notify").Return().Maybe()
	pub := eventpublisher.New(relay, testLogger)

	return testUseCases{
		subscribe:   subscribe.New(repos, subs, gh, testTokenizer, urls, tx, pub, testLogger),
		confirm:     confirm.New(subs, testTokenizer, tx, pub, testLogger),
		unsubscribe: unsubscribe.New(subs, tx, pub, testLogger),
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
	_, err := testPool.Exec(t.Context(), "TRUNCATE subscriptions, repositories, outbox_messages, sagas CASCADE")
	require.NoError(t, err)
}

func confirmTokenFor(t *testing.T, email string) string {
	t.Helper()
	token, err := testTokenizer.Issue(email, testRepoName)
	require.NoError(t, err)

	return token
}

func randomHex64() string {
	return strings.Repeat("ab", 32)
}
