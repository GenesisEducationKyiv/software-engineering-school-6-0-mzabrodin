package config

import (
	"fmt"
	"log/slog"

	"github.com/kelseyhightower/envconfig"
)

type ScannerConfig struct {
	GitHubToken string `envconfig:"GITHUB_TOKEN"`
	WorkerCount int    `envconfig:"SCAN_WORKERS"      default:"5"`
	RedisURL    string `envconfig:"REDIS_URL"                         required:"true"`
	GRPCPort    string `envconfig:"SCANNER_GRPC_PORT" default:"50051"`
	HTTPPort    string `envconfig:"SCANNER_HTTP_PORT" default:"8082"`
	LogLevel    string `envconfig:"LOG_LEVEL"         default:"info"`
	TLS         TLSConfig
}

func (c *ScannerConfig) SlogLevel() slog.Level {
	return slogLevel(c.LogLevel)
}

func LoadScanner() (*ScannerConfig, error) {
	var cfg ScannerConfig
	if err := envconfig.Process("", &cfg); err != nil {
		return nil, fmt.Errorf("process scanner config: %w", err)
	}

	return &cfg, nil
}
