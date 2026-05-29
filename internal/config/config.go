package config

import (
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

type Config struct {
	Port         string        `envconfig:"PORT"          default:"8080"`
	BaseURL      string        `envconfig:"BASE_URL"      default:"http://localhost:8080"`
	GitHubToken  string        `envconfig:"GITHUB_TOKEN"`
	ScanInterval time.Duration `envconfig:"SCAN_INTERVAL" default:"10m"`
	DatabaseURL  string        `envconfig:"DATABASE_URL"                                  required:"true"`
	RedisURL     string        `envconfig:"REDIS_URL"                                     required:"true"`
	SMTP         SMTPConfig
	APIKey       string `envconfig:"API_KEY"`
	LogLevel     string `envconfig:"LOG_LEVEL"     default:"info"`
}

func (c *Config) SlogLevel() slog.Level {
	var l slog.Level
	if err := l.UnmarshalText([]byte(c.LogLevel)); err != nil {
		return slog.LevelInfo
	}

	return l
}

func Load(log *slog.Logger) *Config {
	var cfg Config
	if err := envconfig.Process("", &cfg); err != nil {
		log.Error("failed to load config", "error", err)
	}

	return &cfg
}
