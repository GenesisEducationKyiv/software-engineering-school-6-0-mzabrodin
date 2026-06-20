package main

import (
	"github-release-notifier/internal/bootstrap"
	"github-release-notifier/internal/bootstrap/saga"
	"github-release-notifier/internal/infrastructure/config"
)

func main() {
	bootstrap.Main("saga-orchestrator", config.LoadSaga, saga.Run)
}
