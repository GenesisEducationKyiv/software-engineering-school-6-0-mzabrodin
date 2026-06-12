package connectapi

import (
	"context"
	"log/slog"

	"connectrpc.com/connect"

	"github-release-notifier/internal/subscription/grpc/gen/appv1"
	"github-release-notifier/internal/subscription/usecase/confirm"
	"github-release-notifier/internal/subscription/usecase/list"
	"github-release-notifier/internal/subscription/usecase/subscribe"
	"github-release-notifier/internal/subscription/usecase/unsubscribe"
)

type subscriber interface {
	Execute(ctx context.Context, in subscribe.Input) (subscribe.Output, error)
}

type confirmer interface {
	Execute(ctx context.Context, in confirm.Input) (confirm.Output, error)
}

type unsubscriber interface {
	Execute(ctx context.Context, in unsubscribe.Input) (unsubscribe.Output, error)
}

type lister interface {
	Execute(ctx context.Context, in list.Input) (list.Output, error)
}

type Service struct {
	subscribe   subscriber
	confirm     confirmer
	unsubscribe unsubscriber
	list        lister
	log         *slog.Logger
}

func NewService(
	sub subscriber,
	conf confirmer,
	unsub unsubscriber,
	lst lister,
	log *slog.Logger,
) *Service {
	return &Service{
		subscribe:   sub,
		confirm:     conf,
		unsubscribe: unsub,
		list:        lst,
		log:         log.With("component", "connect-handler"),
	}
}

func (s *Service) Subscribe(
	ctx context.Context,
	req *connect.Request[appv1.SubscribeRequest],
) (*connect.Response[appv1.SubscribeResponse], error) {
	_, err := s.subscribe.Execute(ctx, subscribe.Input{Email: req.Msg.GetEmail(), Repo: req.Msg.GetRepo()})
	if err != nil {
		return nil, s.domainError(ctx, err, "email", req.Msg.GetEmail(), "repo", req.Msg.GetRepo())
	}

	return connect.NewResponse(&appv1.SubscribeResponse{
		Message: "subscription successful, confirmation email sent",
	}), nil
}

func (s *Service) Confirm(
	ctx context.Context,
	req *connect.Request[appv1.ConfirmRequest],
) (*connect.Response[appv1.ConfirmResponse], error) {
	_, err := s.confirm.Execute(ctx, confirm.Input{Token: req.Msg.GetToken()})
	if err != nil {
		return nil, s.domainError(ctx, err, "token", req.Msg.GetToken())
	}

	return connect.NewResponse(&appv1.ConfirmResponse{
		Message: "subscription confirmed successfully",
	}), nil
}

func (s *Service) Unsubscribe(
	ctx context.Context,
	req *connect.Request[appv1.UnsubscribeRequest],
) (*connect.Response[appv1.UnsubscribeResponse], error) {
	_, err := s.unsubscribe.Execute(ctx, unsubscribe.Input{Token: req.Msg.GetToken()})
	if err != nil {
		return nil, s.domainError(ctx, err, "token", req.Msg.GetToken())
	}

	return connect.NewResponse(&appv1.UnsubscribeResponse{
		Message: "unsubscribed successfully",
	}), nil
}

func (s *Service) ListSubscriptions(
	ctx context.Context,
	req *connect.Request[appv1.ListSubscriptionsRequest],
) (*connect.Response[appv1.ListSubscriptionsResponse], error) {
	out, err := s.list.Execute(ctx, list.Input{Email: req.Msg.GetEmail()})
	if err != nil {
		return nil, s.domainError(ctx, err, "email", req.Msg.GetEmail())
	}

	return connect.NewResponse(&appv1.ListSubscriptionsResponse{
		Subscriptions: toProtoSubscriptions(out.Views),
	}), nil
}
