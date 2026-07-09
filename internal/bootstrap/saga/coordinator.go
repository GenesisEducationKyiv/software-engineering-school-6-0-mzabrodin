package saga

import (
	"fmt"
	"log/slog"

	"github-release-notifier/internal/infrastructure/config"
	"github-release-notifier/internal/infrastructure/db"
	"github-release-notifier/internal/infrastructure/outbox"
	"github-release-notifier/internal/saga/adapter/compensationclient"
	"github-release-notifier/internal/saga/adapter/eventpublisher"
	"github-release-notifier/internal/saga/adapter/repository"
	"github-release-notifier/internal/saga/usecase/coordinator"
)

const transportGRPC = "grpc"

func newCoordinator(
	cfg *config.SagaConfig,
	repo *repository.SagaRepository,
	relay *outbox.Relay,
	transactor *db.Transactor,
	log *slog.Logger,
) (*coordinator.Coordinator, func(), error) {
	if cfg.CompensateTransport == transportGRPC {
		client, err := compensationclient.Dial(cfg.SubscriptionGRPCAddr)
		if err != nil {
			return nil, nil, fmt.Errorf("build grpc compensator: %w", err)
		}

		log.Info("saga compensation transport selected", "transport", transportGRPC, "addr", cfg.SubscriptionGRPCAddr)

		coord := coordinator.New(repo, coordinator.NewGRPCCompensator(repo, client), log)

		return coord, func() { _ = client.Close() }, nil
	}

	pub := eventpublisher.New(relay, log)
	log.Info("saga compensation transport selected", "transport", "nats")

	coord := coordinator.New(repo, coordinator.NewNATSCompensator(repo, pub, transactor), log)

	return coord, func() {}, nil
}
