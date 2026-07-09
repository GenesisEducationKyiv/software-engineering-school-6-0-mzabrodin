package main

import (
	"github-release-notifier/internal/bootstrap"
	"github-release-notifier/internal/bootstrap/scanner"
	"github-release-notifier/internal/infrastructure/config"
)

func main() {
	bootstrap.Main("scanner", config.LoadScanner, scanner.Run)
}
