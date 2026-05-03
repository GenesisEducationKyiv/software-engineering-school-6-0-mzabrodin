package service

import (
	"context"
	"errors"
	"testing"

	"github-release-notifier/internal/domain"

	"github.com/google/uuid"
)

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

func (m *mockSubRepository) GetConfirmedByRepoID(_ context.Context, _ uuid.UUID) ([]*domain.Subscription, error) {
	return nil, nil
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

func newSvc(
	repos *mockRepoRepository,
	subs *mockSubRepository,
	gh *mockGitHub,
	mailer *mockMailer,
) *SubscriptionService {
	return NewSubscriptionService(repos, subs, gh, mailer, "http://localhost:8080")
}

func TestParseRepo_Valid(t *testing.T) {
	cases := []struct {
		input     string
		wantOwner string
		wantName  string
	}{
		{"golang/go", "golang", "go"},
		{"owner/repo-name", "owner", "repo-name"},
		{"My.Org/My_Repo.123", "My.Org", "My_Repo.123"},
	}

	for _, tc := range cases {
		owner, name, err := parseRepo(tc.input)
		if err != nil {
			t.Errorf("parseRepo(%q) unexpected error: %v", tc.input, err)
			continue
		}

		if owner != tc.wantOwner || name != tc.wantName {
			t.Errorf("parseRepo(%q) = (%q, %q), want (%q, %q)", tc.input, owner, name, tc.wantOwner, tc.wantName)
		}
	}
}

func TestParseRepo_Invalid(t *testing.T) {
	cases := []string{
		"",
		"noslash",
		"/repo",
		"owner/",
		"owner/repo/extra",
		"owner repo",
	}

	for _, tc := range cases {
		_, _, err := parseRepo(tc)
		if !errors.Is(err, ErrInvalidRepo) {
			t.Errorf("parseRepo(%q) = %v, want ErrInvalidRepo", tc, err)
		}
	}
}

func TestSubscribe_InvalidRepoFormat(t *testing.T) {
	svc := newSvc(&mockRepoRepository{}, &mockSubRepository{}, &mockGitHub{}, &mockMailer{})
	err := svc.Subscribe(context.Background(), "user@example.com", "invalid")

	if !errors.Is(err, ErrInvalidRepo) {
		t.Errorf("got %v, want ErrInvalidRepo", err)
	}
}

func TestSubscribe_RepoNotFoundOnGitHub(t *testing.T) {
	svc := newSvc(
		&mockRepoRepository{},
		&mockSubRepository{},
		&mockGitHub{exists: false},
		&mockMailer{},
	)
	err := svc.Subscribe(context.Background(), "user@example.com", "owner/repo")
	if !errors.Is(err, ErrRepoNotFound) {
		t.Errorf("got %v, want ErrRepoNotFound", err)
	}
}

func TestSubscribe_GitHubError(t *testing.T) {
	ghErr := errors.New("network error")
	svc := newSvc(
		&mockRepoRepository{},
		&mockSubRepository{},
		&mockGitHub{err: ghErr},
		&mockMailer{},
	)
	err := svc.Subscribe(context.Background(), "user@example.com", "owner/repo")
	if err == nil || errors.Is(err, ErrRepoNotFound) {
		t.Errorf("expected wrapped GitHub error, got %v", err)
	}
}

func TestSubscribe_NewRepo_CreatesAndSubscribes(t *testing.T) {
	repos := &mockRepoRepository{getErr: domain.ErrNotFound}
	subs := &mockSubRepository{}
	mailer := &mockMailer{}
	svc := newSvc(repos, subs, &mockGitHub{exists: true}, mailer)

	err := svc.Subscribe(context.Background(), "user@example.com", "owner/repo")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if !repos.created {
		t.Error("expected repository to be created")
	}
}

func TestSubscribe_ExistingRepo_SkipsCreate(t *testing.T) {
	existingRepo := &domain.Repository{ID: uuid.New(), Name: "owner/repo"}
	repos := &mockRepoRepository{repo: existingRepo}
	subs := &mockSubRepository{}
	mailer := &mockMailer{}
	svc := newSvc(repos, subs, &mockGitHub{exists: true}, mailer)

	err := svc.Subscribe(context.Background(), "user@example.com", "owner/repo")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if repos.created {
		t.Error("expected repository NOT to be created (it already exists)")
	}
}

func TestSubscribe_AlreadySubscribed(t *testing.T) {
	existingRepo := &domain.Repository{ID: uuid.New(), Name: "owner/repo"}
	repos := &mockRepoRepository{repo: existingRepo}
	subs := &mockSubRepository{createErr: domain.ErrAlreadyExists}
	svc := newSvc(repos, subs, &mockGitHub{exists: true}, &mockMailer{})

	err := svc.Subscribe(context.Background(), "user@example.com", "owner/repo")
	if !errors.Is(err, domain.ErrAlreadyExists) {
		t.Errorf("got %v, want ErrAlreadyExists", err)
	}
}

func TestConfirm_Success(t *testing.T) {
	subs := &mockSubRepository{}
	svc := newSvc(&mockRepoRepository{}, subs, &mockGitHub{}, &mockMailer{})
	if err := svc.Confirm(context.Background(), "sometoken"); err != nil {
		t.Errorf("unexpected error: %v", err)
	}
}

func TestConfirm_NotFound(t *testing.T) {
	subs := &mockSubRepository{confirmErr: domain.ErrNotFound}
	svc := newSvc(&mockRepoRepository{}, subs, &mockGitHub{}, &mockMailer{})
	err := svc.Confirm(context.Background(), "badtoken")
	if !errors.Is(err, domain.ErrNotFound) {
		t.Errorf("got %v, want ErrNotFound", err)
	}
}

func TestUnsubscribe_Success(t *testing.T) {
	subs := &mockSubRepository{}
	svc := newSvc(&mockRepoRepository{}, subs, &mockGitHub{}, &mockMailer{})
	if err := svc.Unsubscribe(context.Background(), "sometoken"); err != nil {
		t.Errorf("unexpected error: %v", err)
	}
}

func TestUnsubscribe_NotFound(t *testing.T) {
	subs := &mockSubRepository{deleteErr: domain.ErrNotFound}
	svc := newSvc(&mockRepoRepository{}, subs, &mockGitHub{}, &mockMailer{})
	err := svc.Unsubscribe(context.Background(), "badtoken")
	if !errors.Is(err, domain.ErrNotFound) {
		t.Errorf("got %v, want ErrNotFound", err)
	}
}

func TestGetByEmail_ReturnsList(t *testing.T) {
	views := []*domain.SubscriptionView{
		{Email: "user@example.com", Repo: "owner/repo", Confirmed: true, LastSeenTag: new("v1.0.0")},
	}
	subs := &mockSubRepository{views: views}
	svc := newSvc(&mockRepoRepository{}, subs, &mockGitHub{}, &mockMailer{})

	result, err := svc.GetByEmail(context.Background(), "user@example.com")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(result) != 1 {
		t.Errorf("got %d results, want 1", len(result))
	}
}

func TestGetByEmail_Error(t *testing.T) {
	subs := &mockSubRepository{viewsErr: errors.New("db error")}
	svc := newSvc(&mockRepoRepository{}, subs, &mockGitHub{}, &mockMailer{})
	_, err := svc.GetByEmail(context.Background(), "user@example.com")
	if err == nil {
		t.Error("expected error, got nil")
	}
}
