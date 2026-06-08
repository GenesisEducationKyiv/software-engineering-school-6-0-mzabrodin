package emailerserver

import (
	"context"
	"log/slog"

	emailerv1 "github-release-notifier/internal/adapter/grpc/gen/emailer/v1"
	"github-release-notifier/internal/entity"
)

type mailer interface {
	SendConfirmation(ctx context.Context, to, repo, confirmURL string)
	SendReleaseNotifications(ctx context.Context, notifications []entity.ReleaseNotification) entity.BatchResult
}

type Server struct {
	emailerv1.UnimplementedEmailerServiceServer

	mailer mailer
	log    *slog.Logger
}

func NewServer(m mailer, log *slog.Logger) *Server {
	return &Server{mailer: m, log: log.With("component", "emailer-handler")}
}

func (s *Server) SendConfirmation(
	ctx context.Context,
	req *emailerv1.SendConfirmationRequest,
) (*emailerv1.SendConfirmationResponse, error) {
	s.mailer.SendConfirmation(ctx, req.GetTo(), req.GetRepo(), req.GetConfirmUrl())

	return &emailerv1.SendConfirmationResponse{}, nil
}

func (s *Server) SendReleaseNotifications(
	ctx context.Context,
	req *emailerv1.SendReleaseNotificationsRequest,
) (*emailerv1.SendReleaseNotificationsResponse, error) {
	result := s.mailer.SendReleaseNotifications(ctx, toEntityNotifications(req.GetNotifications()))

	return &emailerv1.SendReleaseNotificationsResponse{
		Sent:   uint32(result.Sent), //nolint:gosec // Sent is a non-negative count bounded by the batch size
		Failed: result.Failed,
	}, nil
}
