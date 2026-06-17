package certgen

import (
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/pem"
	"fmt"
	"math/big"
	"net"
	"os"
	"path/filepath"
	"time"
)

const validity = 10 * 365 * 24 * time.Hour

func Write(dir string) error {
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return fmt.Errorf("create output dir: %w", err)
	}

	caCert, caKey, err := newCA()
	if err != nil {
		return err
	}

	if err := writeCert(filepath.Join(dir, "ca.crt"), caCert.Raw); err != nil {
		return err
	}

	loopback := []net.IP{net.IPv4(127, 0, 0, 1)}

	scanner := leafTemplate("scanner", []string{"scanner", "localhost"}, loopback)
	if err := writeLeaf(dir, "scanner", scanner, caCert, caKey); err != nil {
		return err
	}

	subscription := leafTemplate("subscription", nil, nil)

	return writeLeaf(dir, "subscription", subscription, caCert, caKey)
}

func newCA() (*x509.Certificate, *ecdsa.PrivateKey, error) {
	key, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		return nil, nil, fmt.Errorf("generate ca key: %w", err)
	}

	sn, err := serial()
	if err != nil {
		return nil, nil, err
	}

	template := &x509.Certificate{
		SerialNumber:          sn,
		Subject:               pkix.Name{CommonName: "github-release-app CA"},
		NotBefore:             time.Now().Add(-time.Hour),
		NotAfter:              time.Now().Add(validity),
		KeyUsage:              x509.KeyUsageCertSign | x509.KeyUsageCRLSign,
		BasicConstraintsValid: true,
		IsCA:                  true,
	}

	der, err := x509.CreateCertificate(rand.Reader, template, template, &key.PublicKey, key)
	if err != nil {
		return nil, nil, fmt.Errorf("create ca cert: %w", err)
	}

	cert, err := x509.ParseCertificate(der)
	if err != nil {
		return nil, nil, fmt.Errorf("parse ca cert: %w", err)
	}

	return cert, key, nil
}

func leafTemplate(cn string, dnsNames []string, ips []net.IP) *x509.Certificate {
	return &x509.Certificate{
		Subject:     pkix.Name{CommonName: cn},
		NotBefore:   time.Now().Add(-time.Hour),
		NotAfter:    time.Now().Add(validity),
		KeyUsage:    x509.KeyUsageDigitalSignature,
		ExtKeyUsage: []x509.ExtKeyUsage{x509.ExtKeyUsageServerAuth, x509.ExtKeyUsageClientAuth},
		DNSNames:    dnsNames,
		IPAddresses: ips,
	}
}

func writeLeaf(dir, name string, template, caCert *x509.Certificate, caKey *ecdsa.PrivateKey) error {
	key, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		return fmt.Errorf("generate %s key: %w", name, err)
	}

	sn, err := serial()
	if err != nil {
		return err
	}
	template.SerialNumber = sn

	der, err := x509.CreateCertificate(rand.Reader, template, caCert, &key.PublicKey, caKey)
	if err != nil {
		return fmt.Errorf("create %s cert: %w", name, err)
	}

	if err := writeCert(filepath.Join(dir, name+".crt"), der); err != nil {
		return err
	}

	return writeKey(filepath.Join(dir, name+".key"), key)
}

func writeCert(path string, der []byte) error {
	block := &pem.Block{Type: "CERTIFICATE", Bytes: der}
	if err := os.WriteFile(path, pem.EncodeToMemory(block), 0o644); err != nil {
		return fmt.Errorf("write %s: %w", path, err)
	}

	return nil
}

func writeKey(path string, key *ecdsa.PrivateKey) error {
	der, err := x509.MarshalPKCS8PrivateKey(key)
	if err != nil {
		return fmt.Errorf("marshal key: %w", err)
	}

	block := &pem.Block{Type: "PRIVATE KEY", Bytes: der}
	if err := os.WriteFile(path, pem.EncodeToMemory(block), 0o600); err != nil {
		return fmt.Errorf("write %s: %w", path, err)
	}

	return nil
}

func serial() (*big.Int, error) {
	limit := new(big.Int).Lsh(big.NewInt(1), 128)

	n, err := rand.Int(rand.Reader, limit)
	if err != nil {
		return nil, fmt.Errorf("generate serial: %w", err)
	}

	return n, nil
}
