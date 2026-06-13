package tlsconfig_test

import (
	"crypto/tls"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/suite"

	"github-release-notifier/internal/infrastructure/certgen"
	"github-release-notifier/internal/infrastructure/tlsconfig"
)

type TLSConfigSuite struct {
	suite.Suite

	dir    string
	caFile string

	serverCert, serverKey string
	clientCert, clientKey string
}

func TestTLSConfigSuite(t *testing.T) {
	suite.Run(t, new(TLSConfigSuite))
}

func (s *TLSConfigSuite) SetupSuite() {
	s.dir = s.T().TempDir()
	s.Require().NoError(certgen.Write(s.dir))

	s.caFile = filepath.Join(s.dir, "ca.crt")
	s.serverCert = filepath.Join(s.dir, "emailer.crt")
	s.serverKey = filepath.Join(s.dir, "emailer.key")
	s.clientCert = filepath.Join(s.dir, "subscription.crt")
	s.clientKey = filepath.Join(s.dir, "subscription.key")
}

func (s *TLSConfigSuite) TestServerTLS() {
	cfg, err := tlsconfig.ServerTLS(s.serverCert, s.serverKey, s.caFile)
	s.Require().NoError(err)
	s.Equal(uint16(tls.VersionTLS13), cfg.MinVersion)
	s.Equal(tls.RequireAndVerifyClientCert, cfg.ClientAuth)
	s.NotNil(cfg.ClientCAs)
	s.Len(cfg.Certificates, 1)
}

func (s *TLSConfigSuite) TestClientTLS() {
	cfg, err := tlsconfig.ClientTLS(s.clientCert, s.clientKey, s.caFile, "emailer")
	s.Require().NoError(err)
	s.Equal(uint16(tls.VersionTLS13), cfg.MinVersion)
	s.Equal("emailer", cfg.ServerName)
	s.NotNil(cfg.RootCAs)
	s.Len(cfg.Certificates, 1)
}

func (s *TLSConfigSuite) TestRoundTripSucceeds() {
	serverCfg, err := tlsconfig.ServerTLS(s.serverCert, s.serverKey, s.caFile)
	s.Require().NoError(err)

	clientCfg, err := tlsconfig.ClientTLS(s.clientCert, s.clientKey, s.caFile, "localhost")
	s.Require().NoError(err)

	state := s.handshake(serverCfg, clientCfg)
	s.Equal(uint16(tls.VersionTLS13), state.Version)
}

func (s *TLSConfigSuite) TestRoundTripFailsWithoutClientCert() {
	serverCfg, err := tlsconfig.ServerTLS(s.serverCert, s.serverKey, s.caFile)
	s.Require().NoError(err)

	clientCfg := &tls.Config{
		RootCAs:    serverCfg.ClientCAs,
		ServerName: "localhost",
		MinVersion: tls.VersionTLS13,
	}

	s.handshakeError(serverCfg, clientCfg)
}

func (s *TLSConfigSuite) TestServerTLSMissingKeyPair() {
	_, err := tlsconfig.ServerTLS(filepath.Join(s.dir, "missing.crt"), s.serverKey, s.caFile)
	s.Require().Error(err)
}

func (s *TLSConfigSuite) TestClientTLSMissingKeyPair() {
	_, err := tlsconfig.ClientTLS(filepath.Join(s.dir, "missing.crt"), s.clientKey, s.caFile, "emailer")
	s.Require().Error(err)
}

func (s *TLSConfigSuite) TestServerTLSBadCA() {
	bad := filepath.Join(s.T().TempDir(), "bad-ca.crt")
	s.Require().NoError(os.WriteFile(bad, []byte("not a certificate"), 0o600))

	_, err := tlsconfig.ServerTLS(s.serverCert, s.serverKey, bad)
	s.Require().Error(err)
}

func (s *TLSConfigSuite) TestClientTLSMissingCA() {
	_, err := tlsconfig.ClientTLS(s.clientCert, s.clientKey, filepath.Join(s.dir, "missing-ca.crt"), "emailer")
	s.Require().Error(err)
}

func (s *TLSConfigSuite) handshake(serverCfg, clientCfg *tls.Config) tls.ConnectionState {
	s.T().Helper()

	lis, err := tls.Listen("tcp", "127.0.0.1:0", serverCfg)
	s.Require().NoError(err)
	defer lis.Close()

	go func() {
		conn, aErr := lis.Accept()
		if aErr != nil {
			return
		}
		defer conn.Close()
		_, _ = conn.Read(make([]byte, 1))
	}()

	dialer := tls.Dialer{Config: clientCfg}
	conn, err := dialer.DialContext(s.T().Context(), "tcp", lis.Addr().String())
	s.Require().NoError(err)
	defer conn.Close()

	return conn.(*tls.Conn).ConnectionState()
}

func (s *TLSConfigSuite) handshakeError(serverCfg, clientCfg *tls.Config) {
	s.T().Helper()

	lis, err := tls.Listen("tcp", "127.0.0.1:0", serverCfg)
	s.Require().NoError(err)
	defer lis.Close()

	go func() {
		conn, aErr := lis.Accept()
		if aErr != nil {
			return
		}
		defer conn.Close()
		_ = conn.(*tls.Conn).HandshakeContext(s.T().Context())
	}()

	dialer := tls.Dialer{Config: clientCfg}
	conn, err := dialer.DialContext(s.T().Context(), "tcp", lis.Addr().String())
	if err != nil {
		return // dial-time handshake failure is the expected outcome
	}
	defer conn.Close()

	var buf [1]byte
	_, readErr := conn.Read(buf[:])
	s.Require().Error(readErr)
}
