package config

import (
	"fmt"
	"log/slog"

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
	HTTPPort string `envconfig:"NOTIFIER_HTTP_PORT" default:"8081"`
	NATSURL  string `envconfig:"NATS_URL"           default:"nats://localhost:4222"`
	LogLevel string `envconfig:"LOG_LEVEL"          default:"info"`
	SMTP     SMTPConfig
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
