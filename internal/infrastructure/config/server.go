package config

import (
	"fmt"
	"log/slog"
	"time"

	"github.com/kelseyhightower/envconfig"
)

type Config struct {
	Port         string        `envconfig:"PORT"          default:"8080"`
	BaseURL      string        `envconfig:"BASE_URL"      default:"http://localhost:8080"`
	GitHubToken  string        `envconfig:"GITHUB_TOKEN"`
	DatabaseURL  string        `envconfig:"DATABASE_URL"                                  required:"true"`
	RedisURL     string        `envconfig:"REDIS_URL"                                     required:"true"`
	ScannerAddr  string        `envconfig:"SCANNER_ADDR"  default:"localhost:50051"`
	EmailerAddr  string        `envconfig:"EMAILER_ADDR"  default:"localhost:50052"`
	ScanInterval time.Duration `envconfig:"SCAN_INTERVAL" default:"10m"`
	APIKey       string        `envconfig:"API_KEY"`
	LogLevel     string        `envconfig:"LOG_LEVEL"     default:"info"`
	TLS          TLSConfig
}

func (c *Config) SlogLevel() slog.Level {
	return slogLevel(c.LogLevel)
}

func Load() (*Config, error) {
	var cfg Config
	if err := envconfig.Process("", &cfg); err != nil {
		return nil, fmt.Errorf("process config: %w", err)
	}

	return &cfg, nil
}
