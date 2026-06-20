package config

import (
	"fmt"
	"log/slog"

	"github.com/kelseyhightower/envconfig"
)

type SagaConfig struct {
	Port        string `envconfig:"SAGA_PORT"    default:"8083"`
	NATSURL     string `envconfig:"NATS_URL"     default:"nats://localhost:4222"`
	DatabaseURL string `envconfig:"DATABASE_URL"                                 required:"true"`
	LogLevel    string `envconfig:"LOG_LEVEL"    default:"info"`
}

func (c *SagaConfig) SlogLevel() slog.Level {
	return slogLevel(c.LogLevel)
}

func LoadSaga() (*SagaConfig, error) {
	var cfg SagaConfig
	if err := envconfig.Process("", &cfg); err != nil {
		return nil, fmt.Errorf("process saga config: %w", err)
	}

	return &cfg, nil
}
