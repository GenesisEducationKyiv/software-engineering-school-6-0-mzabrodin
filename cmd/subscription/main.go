package main

import (
	"github-release-notifier/internal/bootstrap"
	"github-release-notifier/internal/bootstrap/subscription"
	"github-release-notifier/internal/infrastructure/config"
)

func main() {
	bootstrap.Main("subscription", config.LoadSubscription, subscription.Run)
}
