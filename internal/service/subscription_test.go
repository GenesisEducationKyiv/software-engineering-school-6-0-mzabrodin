package service

import (
	"context"
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github-release-notifier/internal/domain"
)

var ctx = context.Background()

type mockRepoRepository struct {
	repo    *domain.Repository
	getErr  error
	created bool
}

func (m *mockRepoRepository) Create(_ context.Context, repo *domain.Repository) error {
	repo.ID = uuid.New()
	m.created = true
	return nil
}

func (m *mockRepoRepository) GetByName(_ context.Context, _ string) (*domain.Repository, error) {
	return m.repo, m.getErr
}

type mockSubRepository struct {
	createErr  error
	views      []*domain.SubscriptionView
	viewsErr   error
	confirmErr error
	deleteErr  error
}

func (m *mockSubRepository) Create(_ context.Context, sub *domain.Subscription) error {
	sub.ID = uuid.New()
	return m.createErr
}

func (m *mockSubRepository) GetByEmail(_ context.Context, _ string) ([]*domain.SubscriptionView, error) {
	return m.views, m.viewsErr
}

func (m *mockSubRepository) Confirm(_ context.Context, _ string) error {
	return m.confirmErr
}

func (m *mockSubRepository) Delete(_ context.Context, _ string) error {
	return m.deleteErr
}

type mockGitHub struct {
	exists bool
	err    error
}

func (m *mockGitHub) RepoExists(_ context.Context, _, _ string) (bool, error) {
	return m.exists, m.err
}

type mockMailer struct {
	sendErr error
	called  bool
}

func (m *mockMailer) SendConfirmation(_, _, _ string) error {
	m.called = true
	return m.sendErr
}

func (m *mockMailer) Shutdown() {}

type mockURLBuilder struct {
	base string
}

func (m *mockURLBuilder) ConfirmURL(token string) string {
	return m.base + "/api/confirm/" + token
}

func newSvc(
	repos *mockRepoRepository,
	subs *mockSubRepository,
	gh *mockGitHub,
	mailer *mockMailer,
) *SubscriptionService {
	return NewSubscriptionService(repos, subs, gh, mailer, &mockURLBuilder{"http://localhost:8080"})
}

// region Subscribe

func TestSubscribe_InvalidEmail(t *testing.T) {
	svc := newSvc(&mockRepoRepository{}, &mockSubRepository{}, &mockGitHub{}, &mockMailer{})
	err := svc.Subscribe(ctx, "not-an-email", "owner/repo")
	assert.ErrorIs(t, err, domain.ErrInvalidEmail)
}

func TestSubscribe_InvalidRepoFormat(t *testing.T) {
	svc := newSvc(&mockRepoRepository{}, &mockSubRepository{}, &mockGitHub{}, &mockMailer{})
	err := svc.Subscribe(ctx, "user@example.com", "invalid")
	assert.ErrorIs(t, err, domain.ErrInvalidRepo)
}

func TestSubscribe_RepoNotFoundOnGitHub(t *testing.T) {
	svc := newSvc(
		&mockRepoRepository{},
		&mockSubRepository{},
		&mockGitHub{exists: false},
		&mockMailer{},
	)

	err := svc.Subscribe(ctx, "user@example.com", "owner/repo")
	assert.ErrorIs(t, err, domain.ErrRepoNotFound)
}

func TestSubscribe_GitHubError(t *testing.T) {
	svc := newSvc(
		&mockRepoRepository{},
		&mockSubRepository{},
		&mockGitHub{err: domain.ErrRateLimited},
		&mockMailer{},
	)

	err := svc.Subscribe(ctx, "user@example.com", "owner/repo")
	assert.Error(t, err)
	assert.NotErrorIs(t, err, domain.ErrRepoNotFound)
}

func TestSubscribe_NewRepo_CreatesAndSubscribes(t *testing.T) {
	repos := &mockRepoRepository{getErr: domain.ErrNotFound}
	mailer := &mockMailer{}
	svc := newSvc(repos, &mockSubRepository{}, &mockGitHub{exists: true}, mailer)

	err := svc.Subscribe(ctx, "user@example.com", "owner/repo")
	require.NoError(t, err)
	assert.True(t, repos.created)
	assert.True(t, mailer.called)
}

func TestSubscribe_RepoGetError_ReturnsError(t *testing.T) {
	repos := &mockRepoRepository{getErr: assert.AnError}
	svc := newSvc(repos, &mockSubRepository{}, &mockGitHub{exists: true}, &mockMailer{})

	err := svc.Subscribe(ctx, "user@example.com", "owner/repo")
	assert.Error(t, err)
	assert.False(t, repos.created)
}

func TestSubscribe_MailerError_StillReturnsNil(t *testing.T) {
	existingRepo := &domain.Repository{ID: uuid.New(), Name: "owner/repo"}
	mailer := &mockMailer{sendErr: assert.AnError}
	svc := newSvc(&mockRepoRepository{repo: existingRepo}, &mockSubRepository{}, &mockGitHub{exists: true}, mailer)

	err := svc.Subscribe(ctx, "user@example.com", "owner/repo")
	require.NoError(t, err)
	assert.True(t, mailer.called)
}

func TestSubscribe_ExistingRepo_SkipsCreate(t *testing.T) {
	existingRepo := &domain.Repository{ID: uuid.New(), Name: "owner/repo"}
	repos := &mockRepoRepository{repo: existingRepo}
	subs := &mockSubRepository{}
	mailer := &mockMailer{}
	svc := newSvc(repos, subs, &mockGitHub{exists: true}, mailer)

	err := svc.Subscribe(ctx, "user@example.com", "owner/repo")
	require.NoError(t, err)
	assert.False(t, repos.created)
}

func TestSubscribe_AlreadySubscribed(t *testing.T) {
	existingRepo := &domain.Repository{ID: uuid.New(), Name: "owner/repo"}
	repos := &mockRepoRepository{repo: existingRepo}
	subs := &mockSubRepository{createErr: domain.ErrAlreadyExists}
	svc := newSvc(repos, subs, &mockGitHub{exists: true}, &mockMailer{})

	err := svc.Subscribe(ctx, "user@example.com", "owner/repo")
	assert.ErrorIs(t, err, domain.ErrAlreadyExists)
}

// endregion Subscribe

func TestConfirm_Success(t *testing.T) {
	svc := newSvc(&mockRepoRepository{}, &mockSubRepository{}, &mockGitHub{}, &mockMailer{})
	assert.NoError(t, svc.Confirm(ctx, "sometoken"))
}

func TestConfirm_NotFound(t *testing.T) {
	subs := &mockSubRepository{confirmErr: domain.ErrNotFound}
	svc := newSvc(&mockRepoRepository{}, subs, &mockGitHub{}, &mockMailer{})
	assert.ErrorIs(t, svc.Confirm(ctx, "badtoken"), domain.ErrNotFound)
}

func TestUnsubscribe_Success(t *testing.T) {
	svc := newSvc(&mockRepoRepository{}, &mockSubRepository{}, &mockGitHub{}, &mockMailer{})
	assert.NoError(t, svc.Unsubscribe(ctx, "sometoken"))
}

func TestUnsubscribe_NotFound(t *testing.T) {
	subs := &mockSubRepository{deleteErr: domain.ErrNotFound}
	svc := newSvc(&mockRepoRepository{}, subs, &mockGitHub{}, &mockMailer{})
	assert.ErrorIs(t, svc.Unsubscribe(ctx, "badtoken"), domain.ErrNotFound)
}

func TestGetByEmail_InvalidEmail(t *testing.T) {
	svc := newSvc(&mockRepoRepository{}, &mockSubRepository{}, &mockGitHub{}, &mockMailer{})
	_, err := svc.GetByEmail(ctx, "not-an-email")
	assert.ErrorIs(t, err, domain.ErrInvalidEmail)
}

func TestGetByEmail_ReturnsList(t *testing.T) {
	views := []*domain.SubscriptionView{
		{Email: "user@example.com", Repo: "owner/repo", Confirmed: true, LastSeenTag: new("v1.0.0")},
	}
	subs := &mockSubRepository{views: views}
	svc := newSvc(&mockRepoRepository{}, subs, &mockGitHub{}, &mockMailer{})

	result, err := svc.GetByEmail(ctx, "user@example.com")
	require.NoError(t, err)
	assert.Len(t, result, 1)
}

func TestGetByEmail_Error(t *testing.T) {
	subs := &mockSubRepository{viewsErr: assert.AnError}
	svc := newSvc(&mockRepoRepository{}, subs, &mockGitHub{}, &mockMailer{})
	_, err := svc.GetByEmail(ctx, "user@example.com")
	assert.Error(t, err)
}
