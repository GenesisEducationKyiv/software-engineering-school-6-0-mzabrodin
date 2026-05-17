package api_test

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/go-chi/chi/v5"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"

	"github-release-notifier/internal/api"
	"github-release-notifier/internal/domain"
)

type mockService struct {
	mock.Mock
}

func (m *mockService) Subscribe(ctx context.Context, email, repo string) error {
	return m.Called(ctx, email, repo).Error(0)
}

func (m *mockService) Confirm(ctx context.Context, token string) error {
	return m.Called(ctx, token).Error(0)
}

func (m *mockService) Unsubscribe(ctx context.Context, token string) error {
	return m.Called(ctx, token).Error(0)
}

func (m *mockService) GetByEmail(ctx context.Context, email string) ([]*domain.SubscriptionView, error) {
	args := m.Called(ctx, email)
	return args.Get(0).([]*domain.SubscriptionView), args.Error(1)
}

// newHandler creates a Handler backed by svc and registers AssertExpectations cleanup.
func newHandler(t *testing.T) (*api.Handler, *mockService) {
	t.Helper()
	svc := &mockService{}
	t.Cleanup(func() { svc.AssertExpectations(t) })
	return api.NewHandler(svc), svc
}

// withToken injects a chi URL param "token" into the request context.
func withToken(r *http.Request, token string) *http.Request {
	rctx := chi.NewRouteContext()
	rctx.URLParams.Add("token", token)
	return r.WithContext(context.WithValue(r.Context(), chi.RouteCtxKey, rctx))
}

// jsonBody encodes v as JSON and returns a reader.
func jsonBody(t *testing.T, v any) *bytes.Buffer {
	t.Helper()
	b, err := json.Marshal(v)
	require.NoError(t, err)
	return bytes.NewBuffer(b)
}

// 64 hex chars
const validToken = "7453d94668d17cf6adfc2b37045347fa14907a007786ed791865d1754b5737f6"

func TestHandlerSubscribe(t *testing.T) {
	cases := []struct {
		name       string
		body       any
		rawBody    string
		setupMock  func(*mockService)
		wantStatus int
	}{
		{
			name:       "success",
			body:       map[string]string{"email": "user@example.com", "repo": "owner/repo"},
			setupMock:  func(s *mockService) { s.On("Subscribe", mock.Anything, "user@example.com", "owner/repo").Return(nil) },
			wantStatus: http.StatusOK,
		},
		{
			name:       "invalid JSON",
			rawBody:    "not-json",
			wantStatus: http.StatusBadRequest,
		},
		{
			name:       "empty email",
			body:       map[string]string{"email": "", "repo": "owner/repo"},
			wantStatus: http.StatusBadRequest,
		},
		{
			name:       "empty repo",
			body:       map[string]string{"email": "user@example.com", "repo": ""},
			wantStatus: http.StatusBadRequest,
		},
		{
			name: "invalid email",
			body: map[string]string{"email": "user@example.com", "repo": "owner/repo"},
			setupMock: func(s *mockService) {
				s.On("Subscribe", mock.Anything, "user@example.com", "owner/repo").Return(domain.ErrInvalidEmail)
			},
			wantStatus: http.StatusBadRequest,
		},
		{
			name: "invalid repo",
			body: map[string]string{"email": "user@example.com", "repo": "owner/repo"},
			setupMock: func(s *mockService) {
				s.On("Subscribe", mock.Anything, "user@example.com", "owner/repo").Return(domain.ErrInvalidRepo)
			},
			wantStatus: http.StatusBadRequest,
		},
		{
			name: "repo not found",
			body: map[string]string{"email": "user@example.com", "repo": "owner/repo"},
			setupMock: func(s *mockService) {
				s.On("Subscribe", mock.Anything, "user@example.com", "owner/repo").Return(domain.ErrRepoNotFound)
			},
			wantStatus: http.StatusNotFound,
		},
		{
			name: "already exists",
			body: map[string]string{"email": "user@example.com", "repo": "owner/repo"},
			setupMock: func(s *mockService) {
				s.On("Subscribe", mock.Anything, "user@example.com", "owner/repo").Return(domain.ErrAlreadyExists)
			},
			wantStatus: http.StatusConflict,
		},
		{
			name: "internal error",
			body: map[string]string{"email": "user@example.com", "repo": "owner/repo"},
			setupMock: func(s *mockService) {
				s.On("Subscribe", mock.Anything, "user@example.com", "owner/repo").Return(assert.AnError)
			},
			wantStatus: http.StatusInternalServerError,
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			h, svc := newHandler(t)
			if tc.setupMock != nil {
				tc.setupMock(svc)
			}

			var bodyReader *bytes.Buffer
			if tc.rawBody != "" {
				bodyReader = bytes.NewBufferString(tc.rawBody)
			} else {
				bodyReader = jsonBody(t, tc.body)
			}

			r := httptest.NewRequest(http.MethodPost, "/api/subscribe", bodyReader)
			r.Header.Set("Content-Type", "application/json")
			w := httptest.NewRecorder()
			h.Subscribe(w, r)

			assert.Equal(t, tc.wantStatus, w.Code)
		})
	}
}

func TestHandlerConfirm(t *testing.T) {
	cases := []struct {
		name       string
		token      string
		setupMock  func(*mockService)
		wantStatus int
	}{
		{
			name:       "success",
			token:      validToken,
			setupMock:  func(s *mockService) { s.On("Confirm", mock.Anything, validToken).Return(nil) },
			wantStatus: http.StatusOK,
		},
		{
			name:       "short token",
			token:      "tooshort",
			wantStatus: http.StatusBadRequest,
		},
		{
			name:       "not found",
			token:      validToken,
			setupMock:  func(s *mockService) { s.On("Confirm", mock.Anything, validToken).Return(domain.ErrNotFound) },
			wantStatus: http.StatusNotFound,
		},
		{
			name:       "internal error",
			token:      validToken,
			setupMock:  func(s *mockService) { s.On("Confirm", mock.Anything, validToken).Return(assert.AnError) },
			wantStatus: http.StatusInternalServerError,
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			h, svc := newHandler(t)
			if tc.setupMock != nil {
				tc.setupMock(svc)
			}

			r := httptest.NewRequest(http.MethodGet, "/api/confirm/"+tc.token, nil)
			r = withToken(r, tc.token)
			w := httptest.NewRecorder()
			h.Confirm(w, r)

			assert.Equal(t, tc.wantStatus, w.Code)
		})
	}
}

func TestHandlerUnsubscribe(t *testing.T) {
	cases := []struct {
		name       string
		token      string
		setupMock  func(*mockService)
		wantStatus int
	}{
		{
			name:       "success",
			token:      validToken,
			setupMock:  func(s *mockService) { s.On("Unsubscribe", mock.Anything, validToken).Return(nil) },
			wantStatus: http.StatusOK,
		},
		{
			name:       "short token",
			token:      "tooshort",
			wantStatus: http.StatusBadRequest,
		},
		{
			name:       "not found",
			token:      validToken,
			setupMock:  func(s *mockService) { s.On("Unsubscribe", mock.Anything, validToken).Return(domain.ErrNotFound) },
			wantStatus: http.StatusNotFound,
		},
		{
			name:       "internal error",
			token:      validToken,
			setupMock:  func(s *mockService) { s.On("Unsubscribe", mock.Anything, validToken).Return(assert.AnError) },
			wantStatus: http.StatusInternalServerError,
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			h, svc := newHandler(t)
			if tc.setupMock != nil {
				tc.setupMock(svc)
			}

			r := httptest.NewRequest(http.MethodGet, "/api/unsubscribe/"+tc.token, nil)
			r = withToken(r, tc.token)
			w := httptest.NewRecorder()
			h.Unsubscribe(w, r)

			assert.Equal(t, tc.wantStatus, w.Code)
		})
	}
}

func TestHandlerGetSubscriptions(t *testing.T) {
	cases := []struct {
		name       string
		email      string
		setupMock  func(*mockService)
		wantStatus int
		wantLen    int
	}{
		{
			name:  "returns subscriptions",
			email: "user@example.com",
			setupMock: func(s *mockService) {
				views := []*domain.SubscriptionView{
					{Email: "user@example.com", Repo: "owner/repo", Confirmed: true},
				}
				s.On("GetByEmail", mock.Anything, "user@example.com").Return(views, nil)
			},
			wantStatus: http.StatusOK,
			wantLen:    1,
		},
		{
			name:       "missing email",
			email:      "",
			wantStatus: http.StatusBadRequest,
		},
		{
			name:  "invalid email",
			email: "notanemail",
			setupMock: func(s *mockService) {
				s.On("GetByEmail", mock.Anything, "notanemail").
					Return([]*domain.SubscriptionView{}, domain.ErrInvalidEmail)
			},
			wantStatus: http.StatusBadRequest,
		},
		{
			name:  "internal error",
			email: "user@example.com",
			setupMock: func(s *mockService) {
				s.On("GetByEmail", mock.Anything, "user@example.com").
					Return([]*domain.SubscriptionView{}, assert.AnError)
			},
			wantStatus: http.StatusInternalServerError,
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			h, svc := newHandler(t)
			if tc.setupMock != nil {
				tc.setupMock(svc)
			}

			url := "/api/subscriptions"
			if tc.email != "" {
				url += "?email=" + tc.email
			}
			r := httptest.NewRequest(http.MethodGet, url, nil)
			w := httptest.NewRecorder()
			h.GetSubscriptions(w, r)

			assert.Equal(t, tc.wantStatus, w.Code)
			if tc.wantLen > 0 {
				var result []map[string]any
				require.NoError(t, json.NewDecoder(w.Body).Decode(&result))
				assert.Len(t, result, tc.wantLen)
			}
		})
	}
}
