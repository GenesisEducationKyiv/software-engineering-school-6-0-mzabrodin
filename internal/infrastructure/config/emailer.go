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

type EmailerConfig struct {
	GRPCPort string `envconfig:"EMAILER_GRPC_PORT" default:"50052"`
	HTTPPort string `envconfig:"EMAILER_HTTP_PORT" default:"8081"`
	LogLevel string `envconfig:"LOG_LEVEL"         default:"info"`
	SMTP     SMTPConfig
	TLS      TLSConfig
}

func (c *EmailerConfig) SlogLevel() slog.Level {
	return slogLevel(c.LogLevel)
}

func LoadEmailer() (*EmailerConfig, error) {
	var cfg EmailerConfig
	if err := envconfig.Process("", &cfg); err != nil {
		return nil, fmt.Errorf("process emailer config: %w", err)
	}

	return &cfg, nil
}
