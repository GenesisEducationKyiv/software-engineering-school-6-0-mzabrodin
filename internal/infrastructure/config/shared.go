package config

import "log/slog"

type TLSConfig struct {
	CertFile string `envconfig:"TLS_CERT_FILE" required:"true"`
	KeyFile  string `envconfig:"TLS_KEY_FILE"  required:"true"`
	CAFile   string `envconfig:"TLS_CA_FILE"   required:"true"`
}

func slogLevel(level string) slog.Level {
	var l slog.Level
	if err := l.UnmarshalText([]byte(level)); err != nil {
		return slog.LevelInfo
	}

	return l
}
