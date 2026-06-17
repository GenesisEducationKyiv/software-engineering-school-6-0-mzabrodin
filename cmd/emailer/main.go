package main

import (
	"github-release-notifier/internal/bootstrap"
	"github-release-notifier/internal/bootstrap/emailer"
	"github-release-notifier/internal/infrastructure/config"
)

func main() {
	bootstrap.Main("emailer", config.LoadEmailer, emailer.Run)
}
