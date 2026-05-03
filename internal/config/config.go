package config

import (
	"log/slog"
	"net/url"
	"os"
	"strconv"
	"time"
)

type SMTPConfig struct {
	Host      string
	Port      int
	User      string
	Password  string
	FromEmail string
}

type Config struct {
	Port         string
	BaseURL      string
	GitHubToken  string
	ScanInterval time.Duration
	DatabaseURL  string
	RedisURL     string
	SMTP         SMTPConfig
	APIKey       string
}

func Load() *Config {
	return &Config{
		Port:         getEnv("PORT", "8080"),
		BaseURL:      getEnv("BASE_URL", "http://localhost:8080"),
		GitHubToken:  getEnv("GITHUB_TOKEN", ""),
		ScanInterval: getEnvDuration("SCAN_INTERVAL", 10*time.Minute),
		DatabaseURL:  buildDatabaseURL(),
		RedisURL:     buildRedisURL(),
		SMTP: SMTPConfig{
			Host:      getEnv("SMTP_HOST", ""),
			Port:      getEnvInt("SMTP_PORT", 587),
			User:      getEnv("SMTP_USER", ""),
			Password:  getEnv("SMTP_PASSWORD", ""),
			FromEmail: getEnv("SMTP_FROM", ""),
		},
		APIKey: getEnv("API_KEY", ""),
	}
}

func buildDatabaseURL() string {
	u := &url.URL{
		Scheme: "postgres",
		User:   url.UserPassword(getEnv("DB_USER", "postgres"), getEnv("DB_PASSWORD", "postgres")),
		Host:   getEnv("DB_HOST", "localhost") + ":" + getEnv("DB_PORT", "5432"),
		Path:   "/" + getEnv("DB_NAME", "github_release_notifier"),
	}

	q := u.Query()
	q.Set("sslmode", getEnv("DB_SSL_MODE", "disable"))
	u.RawQuery = q.Encode()

	return u.String()
}

func buildRedisURL() string {
	host := getEnv("REDIS_HOST", "localhost") + ":" + getEnv("REDIS_PORT", "6379")
	password := getEnv("REDIS_PASSWORD", "")

	u := &url.URL{
		Scheme: "redis",
		Host:   host,
	}

	if password != "" {
		u.User = url.UserPassword("", password)
	}

	return u.String()
}

func getEnv(key string, fallback string) string {
	if value, ok := os.LookupEnv(key); ok {
		return value
	}

	return fallback
}

func getEnvInt(key string, fallback int) int {
	value, ok := os.LookupEnv(key)
	if !ok {
		return fallback
	}

	parsed, err := strconv.Atoi(value)
	if err != nil {
		slog.Warn("invalid int env var", "key", key, "value", value)
		return fallback
	}

	return parsed
}

func getEnvDuration(key string, fallback time.Duration) time.Duration {
	value, ok := os.LookupEnv(key)
	if !ok {
		return fallback
	}

	parsed, err := time.ParseDuration(value)
	if err != nil {
		slog.Warn("invalid duration env var", "key", key, "value", value)
		return fallback
	}

	return parsed
}
