package config

import (
	"fmt"
	"log/slog"
	"time"

	"github.com/kelseyhightower/envconfig"
)

type Config struct {
	Port            string        `envconfig:"PORT"              default:"8080"`
	BaseURL         string        `envconfig:"BASE_URL"          default:"http://localhost:8080"`
	GitHubToken     string        `envconfig:"GITHUB_TOKEN"`
	DatabaseURL     string        `envconfig:"DATABASE_URL"                                      required:"true"`
	RedisURL        string        `envconfig:"REDIS_URL"                                         required:"true"`
	NATSURL         string        `envconfig:"NATS_URL"          default:"nats://localhost:4222"`
	JWTSecret       string        `envconfig:"JWT_SECRET"                                        required:"true"`
	ConfirmTokenTTL time.Duration `envconfig:"CONFIRM_TOKEN_TTL" default:"24h"`
	APIKey          string        `envconfig:"API_KEY"`
	LogLevel        string        `envconfig:"LOG_LEVEL"         default:"info"`
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
