package grpcapi

import (
	"context"
	"log/slog"

	notifierv1 "github-release-notifier/internal/adapter/grpc/gen/notifier/v1"
	"github-release-notifier/internal/usecase/confirm"
	"github-release-notifier/internal/usecase/list"
	"github-release-notifier/internal/usecase/subscribe"
	"github-release-notifier/internal/usecase/unsubscribe"
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

type Server struct {
	notifierv1.UnimplementedSubscriptionServiceServer

	subscribe   subscriber
	confirm     confirmer
	unsubscribe unsubscriber
	list        lister
	log         *slog.Logger
}

func NewServer(
	sub subscriber,
	conf confirmer,
	unsub unsubscriber,
	lst lister,
	log *slog.Logger,
) *Server {
	return &Server{
		subscribe:   sub,
		confirm:     conf,
		unsubscribe: unsub,
		list:        lst,
		log:         log.With("component", "grpc-handler"),
	}
}

func (s *Server) Subscribe(
	ctx context.Context,
	req *notifierv1.SubscribeRequest,
) (*notifierv1.SubscribeResponse, error) {
	_, err := s.subscribe.Execute(ctx, subscribe.Input{Email: req.GetEmail(), Repo: req.GetRepo()})
	if err != nil {
		return nil, s.writeDomainError(ctx, err, "email", req.GetEmail(), "repo", req.GetRepo())
	}

	return &notifierv1.SubscribeResponse{}, nil
}

func (s *Server) Confirm(
	ctx context.Context,
	req *notifierv1.ConfirmRequest,
) (*notifierv1.ConfirmResponse, error) {
	_, err := s.confirm.Execute(ctx, confirm.Input{Token: req.GetToken()})
	if err != nil {
		return nil, s.writeDomainError(ctx, err, "token", req.GetToken())
	}

	return &notifierv1.ConfirmResponse{}, nil
}

func (s *Server) Unsubscribe(
	ctx context.Context,
	req *notifierv1.UnsubscribeRequest,
) (*notifierv1.UnsubscribeResponse, error) {
	_, err := s.unsubscribe.Execute(ctx, unsubscribe.Input{Token: req.GetToken()})
	if err != nil {
		return nil, s.writeDomainError(ctx, err, "token", req.GetToken())
	}

	return &notifierv1.UnsubscribeResponse{}, nil
}

func (s *Server) ListSubscriptions(
	ctx context.Context,
	req *notifierv1.ListSubscriptionsRequest,
) (*notifierv1.ListSubscriptionsResponse, error) {
	out, err := s.list.Execute(ctx, list.Input{Email: req.GetEmail()})
	if err != nil {
		return nil, s.writeDomainError(ctx, err, "email", req.GetEmail())
	}

	return &notifierv1.ListSubscriptionsResponse{
		Subscriptions: toProtoSubscriptions(out.Views),
	}, nil
}
