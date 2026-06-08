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

type TLSConfig struct {
	CertFile string `envconfig:"TLS_CERT_FILE" required:"true"`
	KeyFile  string `envconfig:"TLS_KEY_FILE"  required:"true"`
	CAFile   string `envconfig:"TLS_CA_FILE"   required:"true"`
}

type Config struct {
	HTTPPort          string        `envconfig:"HTTP_PORT"           default:"8080"`
	GRPCPort          string        `envconfig:"GRPC_PORT"           default:"50051"`
	BaseURL           string        `envconfig:"BASE_URL"            default:"http://localhost:8080"`
	GitHubToken       string        `envconfig:"GITHUB_TOKEN"`
	ScanInterval      time.Duration `envconfig:"SCAN_INTERVAL"       default:"10m"`
	WorkerCount       int           `envconfig:"SCAN_WORKERS"        default:"5"`
	DatabaseURL       string        `envconfig:"DATABASE_URL"                                        required:"true"`
	RedisURL          string        `envconfig:"REDIS_URL"                                           required:"true"`
	EmailerAddr       string        `envconfig:"EMAILER_ADDR"        default:"localhost:50052"`
	EmailerServerName string        `envconfig:"EMAILER_SERVER_NAME" default:"emailer"`
	APIKey            string        `envconfig:"API_KEY"`
	LogLevel          string        `envconfig:"LOG_LEVEL"           default:"info"`
	TLS               TLSConfig
}

type EmailerConfig struct {
	GRPCPort string `envconfig:"EMAILER_GRPC_PORT" default:"50052"`
	HTTPPort string `envconfig:"EMAILER_HTTP_PORT" default:"8081"`
	LogLevel string `envconfig:"LOG_LEVEL"         default:"info"`
	SMTP     SMTPConfig
	TLS      TLSConfig
}

func (c *Config) SlogLevel() slog.Level {
	return slogLevel(c.LogLevel)
}

func (c *EmailerConfig) SlogLevel() slog.Level {
	return slogLevel(c.LogLevel)
}

func slogLevel(level string) slog.Level {
	var l slog.Level
	if err := l.UnmarshalText([]byte(level)); err != nil {
		return slog.LevelInfo
	}

	return l
}

func Load() (*Config, error) {
	var cfg Config
	if err := envconfig.Process("", &cfg); err != nil {
		return nil, fmt.Errorf("process config: %w", err)
	}

	return &cfg, nil
}

func LoadEmailer() (*EmailerConfig, error) {
	var cfg EmailerConfig
	if err := envconfig.Process("", &cfg); err != nil {
		return nil, fmt.Errorf("process emailer config: %w", err)
	}

	return &cfg, nil
}
