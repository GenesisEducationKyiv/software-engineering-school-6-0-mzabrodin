package grcp_api

import (
	"context"
	"log/slog"

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

type Server struct {
	appv1.UnimplementedSubscriptionServiceServer

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
	req *appv1.SubscribeRequest,
) (*appv1.SubscribeResponse, error) {
	_, err := s.subscribe.Execute(ctx, subscribe.Input{Email: req.GetEmail(), Repo: req.GetRepo()})
	if err != nil {
		return nil, s.writeDomainError(ctx, err, "email", req.GetEmail(), "repo", req.GetRepo())
	}

	return &appv1.SubscribeResponse{}, nil
}

func (s *Server) Confirm(
	ctx context.Context,
	req *appv1.ConfirmRequest,
) (*appv1.ConfirmResponse, error) {
	_, err := s.confirm.Execute(ctx, confirm.Input{Token: req.GetToken()})
	if err != nil {
		return nil, s.writeDomainError(ctx, err, "token", req.GetToken())
	}

	return &appv1.ConfirmResponse{}, nil
}

func (s *Server) Unsubscribe(
	ctx context.Context,
	req *appv1.UnsubscribeRequest,
) (*appv1.UnsubscribeResponse, error) {
	_, err := s.unsubscribe.Execute(ctx, unsubscribe.Input{Token: req.GetToken()})
	if err != nil {
		return nil, s.writeDomainError(ctx, err, "token", req.GetToken())
	}

	return &appv1.UnsubscribeResponse{}, nil
}

func (s *Server) ListSubscriptions(
	ctx context.Context,
	req *appv1.ListSubscriptionsRequest,
) (*appv1.ListSubscriptionsResponse, error) {
	out, err := s.list.Execute(ctx, list.Input{Email: req.GetEmail()})
	if err != nil {
		return nil, s.writeDomainError(ctx, err, "email", req.GetEmail())
	}

	return &appv1.ListSubscriptionsResponse{
		Subscriptions: toProtoSubscriptions(out.Views),
	}, nil
}
