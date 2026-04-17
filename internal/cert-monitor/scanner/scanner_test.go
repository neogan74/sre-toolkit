package scanner

import (
	"context"
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/tls"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/pem"
	"math/big"
	"net"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestParseTarget(t *testing.T) {
	tests := []struct {
		input    string
		wantHost string
		wantPort string
	}{
		{"example.com", "example.com", "443"},
		{"example.com:8443", "example.com", "8443"},
		{"https://example.com", "example.com", "443"},
		{"https://example.com:8443", "example.com", "8443"},
		{"http://example.com", "example.com", "443"},
		{"https://example.com/some/path", "example.com", "443"},
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			host, port, err := parseTarget(tt.input)
			require.NoError(t, err)
			assert.Equal(t, tt.wantHost, host)
			assert.Equal(t, tt.wantPort, port)
		})
	}
}

func TestDefaultConfig(t *testing.T) {
	cfg := DefaultConfig()
	assert.Equal(t, 30, cfg.WarnThreshold)
	assert.Equal(t, 7, cfg.CriticalThreshold)
	assert.Equal(t, 10, cfg.Concurrency)
	assert.Equal(t, 10*time.Second, cfg.Timeout)
}

func TestFillCertInfo_OK(t *testing.T) {
	cert := makeCert(t, time.Now().Add(-24*time.Hour), time.Now().Add(60*24*time.Hour))
	cfg := DefaultConfig()
	info := &CertInfo{}
	fillCertInfo(info, cert, cfg)

	assert.Equal(t, StatusOK, info.Status)
	assert.Greater(t, info.DaysLeft, 30)
}

func TestFillCertInfo_Warning(t *testing.T) {
	cert := makeCert(t, time.Now().Add(-24*time.Hour), time.Now().Add(20*24*time.Hour))
	cfg := DefaultConfig()
	info := &CertInfo{}
	fillCertInfo(info, cert, cfg)

	assert.Equal(t, StatusWarning, info.Status)
	// DaysLeft may be 19 or 20 depending on sub-day precision
	assert.InDelta(t, 20, info.DaysLeft, 1)
}

func TestFillCertInfo_Critical(t *testing.T) {
	cert := makeCert(t, time.Now().Add(-24*time.Hour), time.Now().Add(5*24*time.Hour))
	cfg := DefaultConfig()
	info := &CertInfo{}
	fillCertInfo(info, cert, cfg)

	assert.Equal(t, StatusCritical, info.Status)
	// DaysLeft may be 4 or 5 depending on sub-day precision
	assert.InDelta(t, 5, info.DaysLeft, 1)
}

func TestFillCertInfo_Expired(t *testing.T) {
	cert := makeCert(t, time.Now().Add(-48*time.Hour), time.Now().Add(-24*time.Hour))
	cfg := DefaultConfig()
	info := &CertInfo{}
	fillCertInfo(info, cert, cfg)

	assert.Equal(t, StatusExpired, info.Status)
	assert.Less(t, info.DaysLeft, 0)
}

func TestScanURL_RealServer(t *testing.T) {
	// Start a test TLS server with a self-signed cert
	cert := makeCert(t, time.Now().Add(-24*time.Hour), time.Now().Add(60*24*time.Hour))
	tlsCert := certToTLSCert(t, cert)

	srv := httptest.NewUnstartedServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))
	srv.TLS = &tls.Config{Certificates: []tls.Certificate{tlsCert}}
	srv.StartTLS()
	defer srv.Close()

	// Extract host:port from server URL
	addr := strings.TrimPrefix(srv.URL, "https://")

	cfg := DefaultConfig()
	cfg.InsecureSkipVerify = true // self-signed cert

	ctx := context.Background()
	info := ScanURL(ctx, addr, cfg)

	assert.NotNil(t, info)
	assert.Empty(t, info.Error, "unexpected error: %s", info.Error)
	assert.NotEqual(t, StatusError, info.Status)
}

func TestScanURL_InvalidHost(t *testing.T) {
	cfg := DefaultConfig()
	cfg.Timeout = 1 * time.Second

	ctx := context.Background()
	info := ScanURL(ctx, "invalid.localhost.nonexistent", cfg)

	assert.Equal(t, StatusError, info.Status)
	assert.NotEmpty(t, info.Error)
}

func TestScanURLs_Concurrent(t *testing.T) {
	// Scan multiple invalid targets concurrently — should all return errors, not panic
	cfg := DefaultConfig()
	cfg.Timeout = 1 * time.Second
	cfg.Concurrency = 3

	targets := []string{
		"host1.invalid",
		"host2.invalid",
		"host3.invalid",
	}

	ctx := context.Background()
	results := ScanURLs(ctx, targets, cfg)

	assert.Len(t, results, 3)
	for _, r := range results {
		assert.Equal(t, StatusError, r.Status)
	}
}

func TestScanCertificate(t *testing.T) {
	cert := makeCert(t, time.Now().Add(-24*time.Hour), time.Now().Add(90*24*time.Hour))
	cfg := DefaultConfig()

	info := ScanCertificate(cert, "test-source", cfg)

	assert.Equal(t, "test-source", info.Host)
	assert.Equal(t, StatusOK, info.Status)
	assert.Greater(t, info.DaysLeft, 60)
}

// makeCert generates a self-signed test certificate valid between notBefore and notAfter.
func makeCert(t *testing.T, notBefore, notAfter time.Time) *x509.Certificate {
	t.Helper()

	key, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	require.NoError(t, err)

	template := &x509.Certificate{
		SerialNumber: big.NewInt(1),
		Subject:      pkix.Name{CommonName: "test.example.com"},
		NotBefore:    notBefore,
		NotAfter:     notAfter,
		DNSNames:     []string{"test.example.com"},
		IPAddresses:  []net.IP{net.ParseIP("127.0.0.1")},
	}

	certDER, err := x509.CreateCertificate(rand.Reader, template, template, &key.PublicKey, key)
	require.NoError(t, err)

	cert, err := x509.ParseCertificate(certDER)
	require.NoError(t, err)
	return cert
}

// certToTLSCert converts an x509.Certificate + key to a tls.Certificate for test servers.
func certToTLSCert(t *testing.T, cert *x509.Certificate) tls.Certificate {
	t.Helper()

	key, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	require.NoError(t, err)

	template := &x509.Certificate{
		SerialNumber: big.NewInt(2),
		Subject:      pkix.Name{CommonName: "test.example.com"},
		NotBefore:    cert.NotBefore,
		NotAfter:     cert.NotAfter,
		DNSNames:     cert.DNSNames,
		IPAddresses:  cert.IPAddresses,
	}

	certDER, err := x509.CreateCertificate(rand.Reader, template, template, &key.PublicKey, key)
	require.NoError(t, err)

	keyDER, err := x509.MarshalECPrivateKey(key)
	require.NoError(t, err)

	certPEM := pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: certDER})
	keyPEM := pem.EncodeToMemory(&pem.Block{Type: "EC PRIVATE KEY", Bytes: keyDER})

	tlsCert, err := tls.X509KeyPair(certPEM, keyPEM)
	require.NoError(t, err)
	return tlsCert
}
