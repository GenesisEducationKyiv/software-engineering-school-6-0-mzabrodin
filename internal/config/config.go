package config

import (
	"os"
	"time"
)

type SMTPConfig struct {
	Host      string
	Port      string
	User      string
	Password  string
	FromEmail string
}

type Config struct {
	Port         string
	GitHubToken  string
	ScanInterval time.Duration
	DatabaseURL  string
	RedisURL     string
	SMTP         SMTPConfig
	APIKey       string
}

func Load() *Config {
	scanInterval, err := time.ParseDuration(getEnv("SCAN_INTERVAL", "10m"))
	if err != nil {
		scanInterval = 10 * time.Minute
	}

	return &Config{
		Port:         getEnv("PORT", "8080"),
		GitHubToken:  getEnv("GITHUB_TOKEN", ""),
		ScanInterval: scanInterval,
		DatabaseURL:  buildDatabaseURL(),
		RedisURL:     buildRedisURL(),
		SMTP: SMTPConfig{
			Host:      getEnv("SMTP_HOST", ""),
			Port:      getEnv("SMTP_PORT", "587"),
			User:      getEnv("SMTP_USER", ""),
			Password:  getEnv("SMTP_PASSWORD", ""),
			FromEmail: getEnv("SMTP_FROM_EMAIL", ""),
		},
		APIKey: getEnv("API_KEY", ""),
	}
}

func buildDatabaseURL() string {
	return "host=" + getEnv("DB_HOST", "localhost") +
		" port=" + getEnv("DB_PORT", "5432") +
		" user=" + getEnv("DB_USER", "postgres") +
		" password=" + getEnv("DB_PASSWORD", "postgres") +
		" dbname=" + getEnv("DB_NAME", "github_release_notifier") +
		" sslmode=" + getEnv("DB_SSL_MODE", "disable")
}

func buildRedisURL() string {
	password := getEnv("REDIS_PASSWORD", "")
	if password != "" {
		return "redis://:" + password + "@" +
			getEnv("REDIS_HOST", "localhost") + ":" +
			getEnv("REDIS_PORT", "6379")
	}

	return "redis://" +
		getEnv("REDIS_HOST", "localhost") + ":" +
		getEnv("REDIS_PORT", "6379")
}

func getEnv(key string, fallback string) string {
	if value, ok := os.LookupEnv(key); ok {
		return value
	}

	return fallback
}
