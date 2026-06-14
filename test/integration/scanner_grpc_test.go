//go:build integration

package integration

import (
	"net"
	"net/http"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
	"github.com/stretchr/testify/suite"
	"golang.org/x/net/http2"

	"github-release-notifier/internal/infrastructure/certgen"
	"github-release-notifier/internal/infrastructure/tlsconfig"
	"github-release-notifier/internal/scanner/adapter/scannerclient"
	"github-release-notifier/internal/scanner/adapter/scannerserver"
	scanneruc "github-release-notifier/internal/scanner/usecase/scanner"
	"github-release-notifier/internal/shared/entity"
	"github-release-notifier/internal/shared/github"
)

type ScannerGRPCSuite struct {
	suite.Suite

	github *mockGitHub
	client *scannerclient.Client
}

func TestScannerGRPCSuite(t *testing.T) {
	suite.Run(t, new(ScannerGRPCSuite))
}

func (s *ScannerGRPCSuite) SetupTest() {
	dir := s.T().TempDir()
	s.Require().NoError(certgen.Write(dir))

	serverCfg, err := tlsconfig.ServerTLS(
		filepath.Join(dir, "scanner.crt"),
		filepath.Join(dir, "scanner.key"),
		filepath.Join(dir, "ca.crt"),
	)
	s.Require().NoError(err)

	clientCfg, err := tlsconfig.ClientTLS(
		filepath.Join(dir, "subscription.crt"),
		filepath.Join(dir, "subscription.key"),
		filepath.Join(dir, "ca.crt"),
		"localhost",
	)
	s.Require().NoError(err)

	s.github = &mockGitHub{}
	fetcher := scanneruc.New(s.github, 2, testLogger)
	handler, err := scannerserver.NewHandler(scannerserver.NewServer(fetcher, testLogger), testLogger)
	s.Require().NoError(err)

	protocols := new(http.Protocols)
	protocols.SetHTTP1(true)
	protocols.SetHTTP2(true)

	srv := &http.Server{Handler: handler, TLSConfig: serverCfg, Protocols: protocols}

	var lc net.ListenConfig
	lis, err := lc.Listen(s.T().Context(), "tcp", "127.0.0.1:0")
	s.Require().NoError(err)

	go func() { _ = srv.ServeTLS(lis, "", "") }()

	_, port, err := net.SplitHostPort(lis.Addr().String())
	s.Require().NoError(err)

	transport := &http2.Transport{TLSClientConfig: clientCfg}
	httpClient := &http.Client{Transport: transport}
	s.client = scannerclient.New(httpClient, "https://localhost:"+port, transport.CloseIdleConnections, testLogger)

	s.T().Cleanup(func() {
		require.NoError(s.T(), s.client.Close())
		_ = srv.Close()
	})
}

func (s *ScannerGRPCSuite) TestScan() {
	s.github.On("GetLatestRelease", mock.Anything, "owner", "repo1").
		Return(&entity.Release{TagName: "v1.0.0", HTMLURL: "https://github.com/owner/repo1/releases/tag/v1.0.0"}, nil)
	s.github.On("GetLatestRelease", mock.Anything, "owner", "repo2").
		Return(nil, github.ErrNoRelease)
	defer s.github.AssertExpectations(s.T())

	observed, err := s.client.Scan(s.T().Context(), []string{"owner/repo1", "owner/repo2"})
	s.Require().NoError(err)

	s.Require().Len(observed, 1)
	s.Equal("owner/repo1", observed[0].Repo)
	s.Equal("v1.0.0", observed[0].Release.TagName)
	s.Equal("https://github.com/owner/repo1/releases/tag/v1.0.0", observed[0].Release.HTMLURL)
}
