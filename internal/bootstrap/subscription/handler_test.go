package subscription_test

import (
	"context"
	"crypto/tls"
	"encoding/json"
	"io"
	"log/slog"
	"net"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"connectrpc.com/connect"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/suite"
	"golang.org/x/net/http2"

	"github-release-notifier/internal/bootstrap/subscription"
	"github-release-notifier/internal/shared/entity"
	"github-release-notifier/internal/shared/github"
	connectapi "github-release-notifier/internal/subscription/adapter/connectrpc"
	"github-release-notifier/internal/subscription/domain"
	"github-release-notifier/internal/subscription/grpc/gen/subscriptionv1"
	"github-release-notifier/internal/subscription/grpc/gen/subscriptionv1/subscriptionv1connect"
	"github-release-notifier/internal/subscription/usecase/confirm"
	"github-release-notifier/internal/subscription/usecase/list"
	"github-release-notifier/internal/subscription/usecase/subscribe"
	"github-release-notifier/internal/subscription/usecase/unsubscribe"
)

const (
	testKey          = "test-key"
	subscribeMessage = "subscription successful, confirmation email sent"
)

var (
	testLogger = slog.New(slog.NewTextHandler(io.Discard, nil))
	validToken = strings.Repeat("a", 64)
)

type mockSubscriber struct{ mock.Mock }

func (m *mockSubscriber) Execute(ctx context.Context, in subscribe.Input) (subscribe.Output, error) {
	args := m.Called(ctx, in)
	out, _ := args.Get(0).(subscribe.Output)
	return out, args.Error(1)
}

type mockConfirmer struct{ mock.Mock }

func (m *mockConfirmer) Execute(ctx context.Context, in confirm.Input) (confirm.Output, error) {
	args := m.Called(ctx, in)
	out, _ := args.Get(0).(confirm.Output)
	return out, args.Error(1)
}

type mockUnsubscriber struct{ mock.Mock }

func (m *mockUnsubscriber) Execute(ctx context.Context, in unsubscribe.Input) (unsubscribe.Output, error) {
	args := m.Called(ctx, in)
	out, _ := args.Get(0).(unsubscribe.Output)
	return out, args.Error(1)
}

type mockLister struct{ mock.Mock }

func (m *mockLister) Execute(ctx context.Context, in list.Input) (list.Output, error) {
	args := m.Called(ctx, in)
	out, _ := args.Get(0).(list.Output)
	return out, args.Error(1)
}

type HandlerSuite struct {
	suite.Suite

	subscribe   *mockSubscriber
	confirm     *mockConfirmer
	unsubscribe *mockUnsubscriber
	list        *mockLister
	server      *httptest.Server
}

func TestHandlerSuite(t *testing.T) {
	suite.Run(t, new(HandlerSuite))
}

func (s *HandlerSuite) SetupTest() {
	s.subscribe = &mockSubscriber{}
	s.confirm = &mockConfirmer{}
	s.unsubscribe = &mockUnsubscriber{}
	s.list = &mockLister{}

	svc := connectapi.NewService(s.subscribe, s.confirm, s.unsubscribe, s.list, testLogger)
	handler, err := subscription.NewHandler(svc, testKey, testLogger)
	s.Require().NoError(err)

	ts := httptest.NewUnstartedServer(handler)
	ts.Config.Protocols = new(http.Protocols)
	ts.Config.Protocols.SetHTTP1(true)
	ts.Config.Protocols.SetUnencryptedHTTP2(true)
	ts.Start()

	s.server = ts
}

func (s *HandlerSuite) TearDownTest() {
	s.server.Close()
	s.subscribe.AssertExpectations(s.T())
	s.confirm.AssertExpectations(s.T())
	s.unsubscribe.AssertExpectations(s.T())
	s.list.AssertExpectations(s.T())
}

func (s *HandlerSuite) doREST(method, path, body, key string) *http.Response {
	var reader io.Reader = http.NoBody
	if body != "" {
		reader = strings.NewReader(body)
	}

	req, err := http.NewRequestWithContext(s.T().Context(), method, s.server.URL+path, reader)
	s.Require().NoError(err)

	if body != "" {
		req.Header.Set("Content-Type", "application/json")
	}
	if key != "" {
		req.Header.Set("Authorization", "Bearer "+key)
	}

	resp, err := http.DefaultClient.Do(req)
	s.Require().NoError(err)
	s.T().Cleanup(func() { _ = resp.Body.Close() })

	return resp
}

func (s *HandlerSuite) connectClient() subscriptionv1connect.SubscriptionServiceClient {
	httpClient := &http.Client{
		Transport: &http2.Transport{
			AllowHTTP: true,
			DialTLSContext: func(ctx context.Context, network, addr string, _ *tls.Config) (net.Conn, error) {
				var d net.Dialer
				return d.DialContext(ctx, network, addr)
			},
		},
	}

	return subscriptionv1connect.NewSubscriptionServiceClient(httpClient, s.server.URL, connect.WithGRPC())
}

func (s *HandlerSuite) TestREST_Subscribe_Success() {
	s.subscribe.On("Execute", mock.Anything, mock.Anything).Return(subscribe.Output{}, nil)

	resp := s.doREST(http.MethodPost, "/api/subscribe",
		`{"email":"user@example.com","repo":"owner/repo"}`, testKey)

	s.Require().Equal(http.StatusOK, resp.StatusCode)

	var body struct {
		Message string `json:"message"`
	}
	s.Require().NoError(json.NewDecoder(resp.Body).Decode(&body))
	s.Equal(subscribeMessage, body.Message)
}

func (s *HandlerSuite) TestREST_Subscribe_NoKey_Unauthorized() {
	resp := s.doREST(http.MethodPost, "/api/subscribe",
		`{"email":"user@example.com","repo":"owner/repo"}`, "")

	s.Equal(http.StatusUnauthorized, resp.StatusCode)
}

func (s *HandlerSuite) TestREST_Subscribe_WrongKey_Unauthorized() {
	resp := s.doREST(http.MethodPost, "/api/subscribe",
		`{"email":"user@example.com","repo":"owner/repo"}`, "nope")

	s.Equal(http.StatusUnauthorized, resp.StatusCode)
}

func (s *HandlerSuite) TestREST_Subscribe_InvalidEmail_BadRequest() {
	resp := s.doREST(http.MethodPost, "/api/subscribe", `{"email":"bad","repo":"owner/repo"}`, testKey)

	s.Equal(http.StatusBadRequest, resp.StatusCode)
}

func (s *HandlerSuite) TestREST_Subscribe_InvalidRepo_BadRequest() {
	resp := s.doREST(http.MethodPost, "/api/subscribe", `{"email":"user@example.com","repo":"noslash"}`, testKey)

	s.Equal(http.StatusBadRequest, resp.StatusCode)
}

func (s *HandlerSuite) TestREST_Subscribe_RepoNotFound_NotFound() {
	s.subscribe.On("Execute", mock.Anything, mock.Anything).Return(subscribe.Output{}, github.ErrRepoNotFound)

	resp := s.doREST(http.MethodPost, "/api/subscribe",
		`{"email":"user@example.com","repo":"owner/repo"}`, testKey)

	s.Equal(http.StatusNotFound, resp.StatusCode)
}

func (s *HandlerSuite) TestREST_Subscribe_Duplicate_Conflict() {
	s.subscribe.On("Execute", mock.Anything, mock.Anything).Return(subscribe.Output{}, entity.ErrAlreadyExists)

	resp := s.doREST(http.MethodPost, "/api/subscribe",
		`{"email":"user@example.com","repo":"owner/repo"}`, testKey)

	s.Equal(http.StatusConflict, resp.StatusCode)
}

func (s *HandlerSuite) TestREST_Confirm() {
	s.confirm.On("Execute", mock.Anything, mock.Anything).Return(confirm.Output{}, nil)

	bad := s.doREST(http.MethodGet, "/api/confirm/tooshort", "", "")
	s.Equal(http.StatusBadRequest, bad.StatusCode)

	ok := s.doREST(http.MethodGet, "/api/confirm/"+validToken, "", "")
	s.Equal(http.StatusOK, ok.StatusCode)
}

func (s *HandlerSuite) TestREST_List_WrappedShape() {
	s.list.On("Execute", mock.Anything, mock.Anything).Return(list.Output{Views: []*domain.SubscriptionView{
		{Email: "user@example.com", Repo: "owner/repo", Confirmed: true, LastSeenTag: new("v1.0.0")},
	}}, nil)

	resp := s.doREST(http.MethodGet, "/api/subscriptions?email=user@example.com", "", testKey)
	s.Require().Equal(http.StatusOK, resp.StatusCode)

	var body struct {
		Subscriptions []struct {
			Email     string `json:"email"`
			Repo      string `json:"repo"`
			Confirmed bool   `json:"confirmed"`
		} `json:"subscriptions"`
	}
	s.Require().NoError(json.NewDecoder(resp.Body).Decode(&body))
	s.Require().Len(body.Subscriptions, 1)
	s.Equal("owner/repo", body.Subscriptions[0].Repo)
	s.True(body.Subscriptions[0].Confirmed)
}

func (s *HandlerSuite) TestREST_List_EmptyRendersArray() {
	s.list.On("Execute", mock.Anything, mock.Anything).Return(list.Output{Views: nil}, nil)

	resp := s.doREST(http.MethodGet, "/api/subscriptions?email=user@example.com", "", testKey)
	s.Require().Equal(http.StatusOK, resp.StatusCode)

	raw, err := io.ReadAll(resp.Body)
	s.Require().NoError(err)
	s.JSONEq(`{"subscriptions":[]}`, string(raw))
}

func (s *HandlerSuite) TestGRPC_Subscribe_Success_MatchesREST() {
	s.subscribe.On("Execute", mock.Anything, mock.Anything).Return(subscribe.Output{}, nil)

	req := connect.NewRequest(&subscriptionv1.SubscribeRequest{Email: "user@example.com", Repo: "owner/repo"})
	req.Header().Set("Authorization", "Bearer "+testKey)

	resp, err := s.connectClient().Subscribe(s.T().Context(), req)
	s.Require().NoError(err)
	s.Equal(subscribeMessage, resp.Msg.GetMessage())
}

func (s *HandlerSuite) TestGRPC_Subscribe_NoKey_Unauthenticated() {
	req := connect.NewRequest(&subscriptionv1.SubscribeRequest{Email: "user@example.com", Repo: "owner/repo"})

	_, err := s.connectClient().Subscribe(s.T().Context(), req)
	s.Equal(connect.CodeUnauthenticated, connect.CodeOf(err))
}
