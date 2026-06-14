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
	"TLS_CERT_FILE": "/certs/subscription.crt",
	"TLS_KEY_FILE":  "/certs/subscription.key",
	"TLS_CA_FILE":   "/certs/ca.crt",
}

// requiredScannerEnv lists every variable marked required:"true" in ScannerConfig.
var requiredScannerEnv = map[string]string{
	"REDIS_URL":     "redis://localhost:6379",
	"TLS_CERT_FILE": "/certs/scanner.crt",
	"TLS_KEY_FILE":  "/certs/scanner.key",
	"TLS_CA_FILE":   "/certs/ca.crt",
}

// requiredEmailerEnv lists every variable marked required:"true" in EmailerConfig.
var requiredEmailerEnv = map[string]string{
	"SMTP_HOST":     "smtp.example.com",
	"SMTP_USER":     "mailer",
	"SMTP_PASSWORD": "secret",
	"SMTP_FROM":     "noreply@example.com",
	"TLS_CERT_FILE": "/certs/emailer.crt",
	"TLS_KEY_FILE":  "/certs/emailer.key",
	"TLS_CA_FILE":   "/certs/ca.crt",
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
	s.Equal(requiredEnv["TLS_CERT_FILE"], cfg.TLS.CertFile)
	s.Equal(requiredEnv["TLS_KEY_FILE"], cfg.TLS.KeyFile)
	s.Equal(requiredEnv["TLS_CA_FILE"], cfg.TLS.CAFile)
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
	s.Equal("localhost:50051", cfg.ScannerAddr)
	s.Equal("localhost:50052", cfg.EmailerAddr)
	s.Equal(10*time.Minute, cfg.ScanInterval)
}

func (s *ConfigSuite) TestLoadScannerDefaults() {
	s.setEnv(requiredScannerEnv)

	cfg, err := config.LoadScanner()
	s.Require().NoError(err)
	s.Require().NotNil(cfg)

	s.Equal(5, cfg.WorkerCount)
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

func (s *ConfigSuite) TestLoadEmailerAllRequiredSet() {
	s.setEnv(requiredEmailerEnv)

	cfg, err := config.LoadEmailer()
	s.Require().NoError(err)
	s.Require().NotNil(cfg)

	s.Equal(requiredEmailerEnv["SMTP_HOST"], cfg.SMTP.Host)
	s.Equal(requiredEmailerEnv["SMTP_FROM"], cfg.SMTP.FromEmail)
	s.Equal(requiredEmailerEnv["TLS_CERT_FILE"], cfg.TLS.CertFile)
	s.Equal(587, cfg.SMTP.Port)
	s.Equal("50052", cfg.GRPCPort)
	s.Equal("8081", cfg.HTTPPort)
}

func (s *ConfigSuite) TestLoadEmailerMissingRequired() {
	for missing := range requiredEmailerEnv {
		s.Run(missing, func() {
			for k, v := range requiredEmailerEnv {
				if k == missing {
					continue
				}
				s.T().Setenv(k, v)
			}

			cfg, err := config.LoadEmailer()
			s.Require().Error(err)
			s.Nil(cfg)
		})
	}
}
