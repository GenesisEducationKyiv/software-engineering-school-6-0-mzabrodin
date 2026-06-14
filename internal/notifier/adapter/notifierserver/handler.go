package notifierserver

import (
	"context"
	"log/slog"

	"connectrpc.com/connect"

	"github-release-notifier/internal/notifier"
	"github-release-notifier/internal/notifier/grpc/gen/notifierv1"
)

type mailer interface {
	SendConfirmation(ctx context.Context, to, repo, confirmURL string)
	SendReleaseNotifications(ctx context.Context, notifications []notifier.ReleaseNotification) notifier.BatchResult
}

type Server struct {
	mailer mailer
	log    *slog.Logger
}

func NewServer(m mailer, log *slog.Logger) *Server {
	return &Server{mailer: m, log: log.With("component", "notifier-handler")}
}

func (s *Server) SendConfirmation(
	ctx context.Context,
	req *connect.Request[notifierv1.SendConfirmationRequest],
) (*connect.Response[notifierv1.SendConfirmationResponse], error) {
	s.mailer.SendConfirmation(ctx, req.Msg.GetTo(), req.Msg.GetRepo(), req.Msg.GetConfirmUrl())

	return connect.NewResponse(&notifierv1.SendConfirmationResponse{}), nil
}

func (s *Server) SendReleaseNotifications(
	ctx context.Context,
	req *connect.Request[notifierv1.SendReleaseNotificationsRequest],
) (*connect.Response[notifierv1.SendReleaseNotificationsResponse], error) {
	result := s.mailer.SendReleaseNotifications(ctx, toEntityNotifications(req.Msg.GetNotifications()))

	return connect.NewResponse(&notifierv1.SendReleaseNotificationsResponse{
		Sent:   uint32(result.Sent), //nolint:gosec // Sent is a non-negative count bounded by the batch size
		Failed: result.Failed,
	}), nil
}
