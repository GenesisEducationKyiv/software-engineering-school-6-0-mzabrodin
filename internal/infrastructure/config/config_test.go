package config_test

import (
	"testing"
	"time"

	"github.com/stretchr/testify/suite"

	"github-release-notifier/internal/infrastructure/config"
)

// requiredEnv lists every variable marked required:"true" in the config structs.
var requiredEnv = map[string]string{
	"DATABASE_URL": "postgres://user:pass@localhost:5432/db",
	"REDIS_URL":    "redis://localhost:6379",
	"JWT_SECRET":   "test-secret",
}

// requiredScannerEnv lists every variable marked required:"true" in ScannerConfig.
var requiredScannerEnv = map[string]string{
	"REDIS_URL":     "redis://localhost:6379",
	"DATABASE_URL":  "postgres://user:pass@localhost:5432/db",
	"TLS_CERT_FILE": "/certs/scanner.crt",
	"TLS_KEY_FILE":  "/certs/scanner.key",
	"TLS_CA_FILE":   "/certs/ca.crt",
}

// requiredNotifierEnv lists every variable marked required:"true" in NotifierConfig.
var requiredNotifierEnv = map[string]string{
	"DATABASE_URL":  "postgres://user:pass@localhost:5432/db",
	"SMTP_HOST":     "smtp.example.com",
	"SMTP_USER":     "mailer",
	"SMTP_PASSWORD": "secret",
	"SMTP_FROM":     "noreply@example.com",
}

type ConfigSuite struct {
	suite.Suite
}

func TestConfigSuite(t *testing.T) {
	suite.Run(t, new(ConfigSuite))
}

func (s *ConfigSuite) setEnv(env map[string]string) {
	for k, v := range env {
		s.T().Setenv(k, v)
	}
}

func (s *ConfigSuite) TestLoadAllRequiredSet() {
	s.setEnv(requiredEnv)

	cfg, err := config.Load()
	s.Require().NoError(err)
	s.Require().NotNil(cfg)

	s.Equal(requiredEnv["DATABASE_URL"], cfg.DatabaseURL)
	s.Equal(requiredEnv["REDIS_URL"], cfg.RedisURL)
	s.Equal(requiredEnv["JWT_SECRET"], cfg.JWTSecret)
}

func (s *ConfigSuite) TestLoadMissingRequired() {
	for missing := range requiredEnv {
		s.Run(missing, func() {
			for k, v := range requiredEnv {
				if k == missing {
					continue
				}
				s.T().Setenv(k, v)
			}

			cfg, err := config.Load()
			s.Require().Error(err)
			s.Nil(cfg)
		})
	}
}

func (s *ConfigSuite) TestLoadDefaults() {
	s.setEnv(requiredEnv)

	cfg, err := config.Load()
	s.Require().NoError(err)

	s.Equal("8080", cfg.Port)
	s.Equal("http://localhost:8080", cfg.BaseURL)
	s.Equal("nats://localhost:4222", cfg.NATSURL)
	s.Equal(24*time.Hour, cfg.ConfirmTokenTTL)
	s.Equal(24*time.Hour, cfg.PendingCleanupInterval)
}

func (s *ConfigSuite) TestLoadScannerDefaults() {
	s.setEnv(requiredScannerEnv)

	cfg, err := config.LoadScanner()
	s.Require().NoError(err)
	s.Require().NotNil(cfg)

	s.Equal(5, cfg.WorkerCount)
	s.Equal(10*time.Minute, cfg.ScanInterval)
	s.Equal("nats://localhost:4222", cfg.NATSURL)
	s.Equal("50051", cfg.GRPCPort)
	s.Equal("8082", cfg.HTTPPort)
}

func (s *ConfigSuite) TestLoadScannerMissingRequired() {
	for missing := range requiredScannerEnv {
		s.Run(missing, func() {
			for k, v := range requiredScannerEnv {
				if k == missing {
					continue
				}
				s.T().Setenv(k, v)
			}

			cfg, err := config.LoadScanner()
			s.Require().Error(err)
			s.Nil(cfg)
		})
	}
}

func (s *ConfigSuite) TestLoadNotifierAllRequiredSet() {
	s.setEnv(requiredNotifierEnv)

	cfg, err := config.LoadNotifier()
	s.Require().NoError(err)
	s.Require().NotNil(cfg)

	s.Equal(requiredNotifierEnv["DATABASE_URL"], cfg.DatabaseURL)
	s.Equal(requiredNotifierEnv["SMTP_HOST"], cfg.SMTP.Host)
	s.Equal(requiredNotifierEnv["SMTP_FROM"], cfg.SMTP.FromEmail)
	s.Equal(587, cfg.SMTP.Port)
	s.Equal("nats://localhost:4222", cfg.NATSURL)
	s.Equal("8081", cfg.HTTPPort)
	s.Equal(15*time.Minute, cfg.RetryInterval)
	s.Equal(5, cfg.MaxRetries)
	s.Equal(24*time.Hour, cfg.ConfirmationTTL)
}

func (s *ConfigSuite) TestLoadNotifierMissingRequired() {
	for missing := range requiredNotifierEnv {
		s.Run(missing, func() {
			for k, v := range requiredNotifierEnv {
				if k == missing {
					continue
				}
				s.T().Setenv(k, v)
			}

			cfg, err := config.LoadNotifier()
			s.Require().Error(err)
			s.Nil(cfg)
		})
	}
}
