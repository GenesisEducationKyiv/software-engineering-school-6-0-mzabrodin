package config_test

import (
	"testing"
	"time"

	"github.com/stretchr/testify/suite"

	"github-release-notifier/internal/infrastructure/config"
)

// requiredEnv lists every variable marked required:"true" in the config structs.
var requiredEnv = map[string]string{
	"DATABASE_URL":  "postgres://user:pass@localhost:5432/db",
	"REDIS_URL":     "redis://localhost:6379",
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

// setEnv sets every entry of env for the duration of the current (sub)test.
// t.Setenv restores the previous value automatically on cleanup.
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
	s.Equal(requiredEnv["SMTP_HOST"], cfg.SMTP.Host)
	s.Equal(requiredEnv["SMTP_USER"], cfg.SMTP.User)
	s.Equal(requiredEnv["SMTP_PASSWORD"], cfg.SMTP.Password)
	s.Equal(requiredEnv["SMTP_FROM"], cfg.SMTP.FromEmail)
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

	s.Equal("8080", cfg.HTTPPort)
	s.Equal("50051", cfg.GRPCPort)
	s.Equal("http://localhost:8080", cfg.BaseURL)
	s.Equal(10*time.Minute, cfg.ScanInterval)
	s.Equal(587, cfg.SMTP.Port)
}
