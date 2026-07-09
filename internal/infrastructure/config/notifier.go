package config

import (
	"fmt"
	"log/slog"
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

type NotifierConfig struct {
	Port            string        `envconfig:"NOTIFIER_PORT"         default:"8081"`
	NATSURL         string        `envconfig:"NATS_URL"              default:"nats://localhost:4222"`
	DatabaseURL     string        `envconfig:"DATABASE_URL"                                          required:"true"`
	BaseURL         string        `envconfig:"BASE_URL"              default:"http://localhost:8080"`
	RetryInterval   time.Duration `envconfig:"RETRY_INTERVAL"        default:"15m"`
	MaxRetries      int           `envconfig:"MAX_RETRIES"           default:"5"`
	ConfirmationTTL time.Duration `envconfig:"CONFIRMATION_TTL"      default:"24h"`
	ProcessedTTL    time.Duration `envconfig:"PROCESSED_RELEASE_TTL" default:"720h"`
	LogLevel        string        `envconfig:"LOG_LEVEL"             default:"info"`
	SMTP            SMTPConfig
}

func (c *NotifierConfig) SlogLevel() slog.Level {
	return slogLevel(c.LogLevel)
}

func LoadNotifier() (*NotifierConfig, error) {
	var cfg NotifierConfig
	if err := envconfig.Process("", &cfg); err != nil {
		return nil, fmt.Errorf("process notifier config: %w", err)
	}

	return &cfg, nil
}
