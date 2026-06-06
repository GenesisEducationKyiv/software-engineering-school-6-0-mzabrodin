package api_test

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/go-chi/chi/v5"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
	"github.com/stretchr/testify/suite"

	api "github-release-notifier/internal/adapter/http"
	"github-release-notifier/internal/entity"
	"github-release-notifier/internal/usecase/confirm"
	"github-release-notifier/internal/usecase/list"
	"github-release-notifier/internal/usecase/subscribe"
	"github-release-notifier/internal/usecase/unsubscribe"
)

type HandlerSuite struct {
	suite.Suite
	handler     *api.Handler
	subscribe   *mockSubscribe
	confirm     *mockConfirm
	unsubscribe *mockUnsubscribe
	list        *mockList
}

func (s *HandlerSuite) SetupSubTest() {
	s.subscribe = &mockSubscribe{}
	s.confirm = &mockConfirm{}
	s.unsubscribe = &mockUnsubscribe{}
	s.list = &mockList{}
	s.handler = api.NewHandler(
		s.subscribe,
		s.confirm,
		s.unsubscribe,
		s.list,
		slog.New(slog.NewTextHandler(io.Discard, nil)),
	)
}

func (s *HandlerSuite) TearDownSubTest() {
	s.subscribe.AssertExpectations(s.T())
	s.confirm.AssertExpectations(s.T())
	s.unsubscribe.AssertExpectations(s.T())
	s.list.AssertExpectations(s.T())
}

func TestHandlerSuite(t *testing.T) {
	suite.Run(t, new(HandlerSuite))
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

func (s *HandlerSuite) TestSubscribe() {
	cases := []struct {
		name       string
		body       map[string]string
		rawBody    string
		setupMock  func(*mockSubscribe)
		wantStatus int
	}{
		{
			name: "success",
			body: map[string]string{"email": "user@example.com", "repo": "owner/repo"},
			setupMock: func(m *mockSubscribe) {
				m.On("Execute", mock.Anything, subscribe.Input{Email: "user@example.com", Repo: "owner/repo"}).
					Return(subscribe.Output{}, nil)
			},
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
			setupMock: func(m *mockSubscribe) {
				m.On("Execute", mock.Anything, subscribe.Input{Email: "user@example.com", Repo: "owner/repo"}).
					Return(subscribe.Output{}, entity.ErrInvalidEmail)
			},
			wantStatus: http.StatusBadRequest,
		},
		{
			name: "invalid repo",
			body: map[string]string{"email": "user@example.com", "repo": "owner/repo"},
			setupMock: func(m *mockSubscribe) {
				m.On("Execute", mock.Anything, subscribe.Input{Email: "user@example.com", Repo: "owner/repo"}).
					Return(subscribe.Output{}, entity.ErrInvalidRepo)
			},
			wantStatus: http.StatusBadRequest,
		},
		{
			name: "repo not found",
			body: map[string]string{"email": "user@example.com", "repo": "owner/repo"},
			setupMock: func(m *mockSubscribe) {
				m.On("Execute", mock.Anything, subscribe.Input{Email: "user@example.com", Repo: "owner/repo"}).
					Return(subscribe.Output{}, entity.ErrRepoNotFound)
			},
			wantStatus: http.StatusNotFound,
		},
		{
			name: "already exists",
			body: map[string]string{"email": "user@example.com", "repo": "owner/repo"},
			setupMock: func(m *mockSubscribe) {
				m.On("Execute", mock.Anything, subscribe.Input{Email: "user@example.com", Repo: "owner/repo"}).
					Return(subscribe.Output{}, entity.ErrAlreadyExists)
			},
			wantStatus: http.StatusConflict,
		},
		{
			name: "internal error",
			body: map[string]string{"email": "user@example.com", "repo": "owner/repo"},
			setupMock: func(m *mockSubscribe) {
				m.On("Execute", mock.Anything, subscribe.Input{Email: "user@example.com", Repo: "owner/repo"}).
					Return(subscribe.Output{}, assert.AnError)
			},
			wantStatus: http.StatusInternalServerError,
		},
	}

	for _, tc := range cases {
		s.Run(tc.name, func() {
			if tc.setupMock != nil {
				tc.setupMock(s.subscribe)
			}

			var bodyReader *bytes.Buffer
			if tc.rawBody != "" {
				bodyReader = bytes.NewBufferString(tc.rawBody)
			} else {
				bodyReader = jsonBody(s.T(), tc.body)
			}

			r := httptest.NewRequestWithContext(s.T().Context(), http.MethodPost, "/api/subscribe", bodyReader)
			r.Header.Set("Content-Type", "application/json")
			w := httptest.NewRecorder()
			s.handler.Subscribe(w, r)

			s.Equal(tc.wantStatus, w.Code)
		})
	}
}

func (s *HandlerSuite) TestConfirm() {
	cases := []struct {
		name       string
		token      string
		setupMock  func(*mockConfirm)
		wantStatus int
	}{
		{
			name:  "success",
			token: validToken,
			setupMock: func(m *mockConfirm) {
				m.On("Execute", mock.Anything, confirm.Input{Token: validToken}).Return(confirm.Output{}, nil)
			},
			wantStatus: http.StatusOK,
		},
		{
			name:       "short token",
			token:      "tooshort",
			wantStatus: http.StatusBadRequest,
		},
		{
			name:  "not found",
			token: validToken,
			setupMock: func(m *mockConfirm) {
				m.On("Execute", mock.Anything, confirm.Input{Token: validToken}).
					Return(confirm.Output{}, entity.ErrNotFound)
			},
			wantStatus: http.StatusNotFound,
		},
		{
			name:  "internal error",
			token: validToken,
			setupMock: func(m *mockConfirm) {
				m.On("Execute", mock.Anything, confirm.Input{Token: validToken}).
					Return(confirm.Output{}, assert.AnError)
			},
			wantStatus: http.StatusInternalServerError,
		},
	}

	for _, tc := range cases {
		s.Run(tc.name, func() {
			if tc.setupMock != nil {
				tc.setupMock(s.confirm)
			}

			r := httptest.NewRequestWithContext(s.T().Context(), http.MethodGet, "/api/confirm/"+tc.token, http.NoBody)
			r = withToken(r, tc.token)
			w := httptest.NewRecorder()
			s.handler.Confirm(w, r)

			s.Equal(tc.wantStatus, w.Code)
		})
	}
}

func (s *HandlerSuite) TestUnsubscribe() {
	cases := []struct {
		name       string
		token      string
		setupMock  func(*mockUnsubscribe)
		wantStatus int
	}{
		{
			name:  "success",
			token: validToken,
			setupMock: func(m *mockUnsubscribe) {
				m.On("Execute", mock.Anything, unsubscribe.Input{Token: validToken}).Return(unsubscribe.Output{}, nil)
			},
			wantStatus: http.StatusOK,
		},
		{
			name:       "short token",
			token:      "tooshort",
			wantStatus: http.StatusBadRequest,
		},
		{
			name:  "not found",
			token: validToken,
			setupMock: func(m *mockUnsubscribe) {
				m.On("Execute", mock.Anything, unsubscribe.Input{Token: validToken}).
					Return(unsubscribe.Output{}, entity.ErrNotFound)
			},
			wantStatus: http.StatusNotFound,
		},
		{
			name:  "internal error",
			token: validToken,
			setupMock: func(m *mockUnsubscribe) {
				m.On("Execute", mock.Anything, unsubscribe.Input{Token: validToken}).
					Return(unsubscribe.Output{}, assert.AnError)
			},
			wantStatus: http.StatusInternalServerError,
		},
	}

	for _, tc := range cases {
		s.Run(tc.name, func() {
			if tc.setupMock != nil {
				tc.setupMock(s.unsubscribe)
			}

			r := httptest.NewRequestWithContext(
				s.T().Context(),
				http.MethodGet,
				"/api/unsubscribe/"+tc.token,
				http.NoBody,
			)
			r = withToken(r, tc.token)
			w := httptest.NewRecorder()
			s.handler.Unsubscribe(w, r)

			s.Equal(tc.wantStatus, w.Code)
		})
	}
}

func (s *HandlerSuite) TestGetSubscriptions() {
	cases := []struct {
		name       string
		email      string
		setupMock  func(*mockList)
		wantStatus int
		wantLen    int
	}{
		{
			name:  "returns subscriptions",
			email: "user@example.com",
			setupMock: func(m *mockList) {
				views := []*entity.SubscriptionView{
					{Email: "user@example.com", Repo: "owner/repo", Confirmed: true},
				}
				m.On("Execute", mock.Anything, list.Input{Email: "user@example.com"}).
					Return(list.Output{Views: views}, nil)
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
			setupMock: func(m *mockList) {
				m.On("Execute", mock.Anything, list.Input{Email: "notanemail"}).
					Return(list.Output{}, entity.ErrInvalidEmail)
			},
			wantStatus: http.StatusBadRequest,
		},
		{
			name:  "internal error",
			email: "user@example.com",
			setupMock: func(m *mockList) {
				m.On("Execute", mock.Anything, list.Input{Email: "user@example.com"}).
					Return(list.Output{}, assert.AnError)
			},
			wantStatus: http.StatusInternalServerError,
		},
	}

	for _, tc := range cases {
		s.Run(tc.name, func() {
			if tc.setupMock != nil {
				tc.setupMock(s.list)
			}

			url := "/api/subscriptions"
			if tc.email != "" {
				url += "?email=" + tc.email
			}
			r := httptest.NewRequestWithContext(s.T().Context(), http.MethodGet, url, http.NoBody)
			w := httptest.NewRecorder()
			s.handler.GetSubscriptions(w, r)

			s.Equal(tc.wantStatus, w.Code)
			if tc.wantLen > 0 {
				var result []map[string]any
				s.Require().NoError(json.NewDecoder(w.Body).Decode(&result))
				s.Len(result, tc.wantLen)
			}
		})
	}
}
