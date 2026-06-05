package config

import (
	"fmt"
	"time"

	"github.com/kelseyhightower/envconfig"
)

type SMTPConfig struct {
	Host      string `envconfig:"SMTP_HOST"     required:"true"`
	Port      int    `envconfig:"SMTP_PORT"                     default:"587"`
	User      string `envconfig:"SMTP_USER"     required:"true"`
	Password  string `envconfig:"SMTP_PASSWORD" required:"true"`
	FromEmail string `envconfig:"SMTP_FROM"     required:"true"`
}

type Config struct {
	Port         string        `envconfig:"PORT"          default:"8080"`
	BaseURL      string        `envconfig:"BASE_URL"      default:"http://localhost:8080"`
	GitHubToken  string        `envconfig:"GITHUB_TOKEN"`
	ScanInterval time.Duration `envconfig:"SCAN_INTERVAL" default:"10m"`
	DatabaseURL  string        `envconfig:"DATABASE_URL"                                  required:"true"`
	RedisURL     string        `envconfig:"REDIS_URL"                                     required:"true"`
	SMTP         SMTPConfig
	APIKey       string `envconfig:"API_KEY"`
}

func Load() (*Config, error) {
	var cfg Config
	if err := envconfig.Process("", &cfg); err != nil {
		return nil, fmt.Errorf("process config: %w", err)
	}

	return &cfg, nil
}
