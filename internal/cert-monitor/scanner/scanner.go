// Package scanner provides TLS certificate scanning functionality.
package scanner

import (
	"context"
	"crypto/tls"
	"crypto/x509"
	"fmt"
	"net"
	"sync"
	"time"
)

// Status represents the health status of a certificate.
type Status string

// Certificate status values.
const (
	StatusOK       Status = "OK"
	StatusWarning  Status = "WARNING"
	StatusCritical Status = "CRITICAL"
	StatusExpired  Status = "EXPIRED"
	StatusError    Status = "ERROR"
)

// CertInfo holds information about a scanned TLS certificate.
type CertInfo struct {
	Host        string
	Port        string
	Subject     string
	Issuer      string
	DNSNames    []string
	NotBefore   time.Time
	NotAfter    time.Time
	DaysLeft    int
	Status      Status
	Error       string
	Serial      string
	Fingerprint string

	// Chain validation results. Populated for URL scans where the full
	// certificate chain presented by the server is available.
	ChainLength int    // number of certificates the server presented (leaf + intermediates)
	ChainValid  bool   // true if the leaf chains to a trusted root and the hostname matches
	ChainError  string // human-readable reason the chain failed verification, if any
	SelfSigned  bool   // true if the leaf certificate is self-signed
}

// Config holds scanner configuration.
type Config struct {
	Timeout            time.Duration
	WarnThreshold      int // days
	CriticalThreshold  int // days
	InsecureSkipVerify bool
	Concurrency        int
}

// DefaultConfig returns scanner config with sensible defaults.
func DefaultConfig() *Config {
	return &Config{
		Timeout:            10 * time.Second,
		WarnThreshold:      30,
		CriticalThreshold:  7,
		InsecureSkipVerify: false,
		Concurrency:        10,
	}
}

// ScanURL scans a single URL/host for TLS certificate information.
func ScanURL(ctx context.Context, target string, cfg *Config) *CertInfo {
	if cfg == nil {
		cfg = DefaultConfig()
	}

	host, port := parseTarget(target)

	info := &CertInfo{
		Host: host,
		Port: port,
	}

	dialer := &tls.Dialer{
		NetDialer: &net.Dialer{
			Timeout: cfg.Timeout,
		},
		Config: &tls.Config{
			InsecureSkipVerify: cfg.InsecureSkipVerify, //nolint:gosec // InsecureSkipVerify is controlled by user configuration
			ServerName:         host,
		},
	}

	conn, err := dialer.DialContext(ctx, "tcp", net.JoinHostPort(host, port))
	if err != nil {
		info.Status = StatusError
		info.Error = err.Error()
		return info
	}
	defer conn.Close()

	tlsConn, ok := conn.(*tls.Conn)
	if !ok {
		info.Status = StatusError
		info.Error = "failed to get TLS connection"
		return info
	}

	certs := tlsConn.ConnectionState().PeerCertificates
	if len(certs) == 0 {
		info.Status = StatusError
		info.Error = "no certificates found"
		return info
	}

	cert := certs[0]
	fillCertInfo(info, cert, cfg)
	verifyChain(info, host, certs, nil)
	return info
}

// verifyChain inspects the full certificate chain presented by a server and
// records the result on info. It builds an intermediate pool from every
// certificate after the leaf and verifies the leaf against roots (system roots
// when roots is nil), matching the requested hostname.
//
// Verification is performed independently of Config.InsecureSkipVerify so that
// chain problems are always reported, even when the scan is otherwise lenient
// about trust (e.g. for self-signed certificates).
func verifyChain(info *CertInfo, host string, certs []*x509.Certificate, roots *x509.CertPool) {
	info.ChainLength = len(certs)
	if len(certs) == 0 {
		return
	}

	leaf := certs[0]
	info.SelfSigned = isSelfSigned(leaf)

	intermediates := x509.NewCertPool()
	for _, c := range certs[1:] {
		intermediates.AddCert(c)
	}

	opts := x509.VerifyOptions{
		DNSName:       host,
		Roots:         roots, // nil => system trust store
		Intermediates: intermediates,
	}

	if _, err := leaf.Verify(opts); err != nil {
		info.ChainValid = false
		info.ChainError = err.Error()
		return
	}
	info.ChainValid = true
}

// isSelfSigned reports whether a certificate is its own issuer. It first checks
// the issuer/subject names cheaply, then confirms with a signature check.
func isSelfSigned(cert *x509.Certificate) bool {
	if cert == nil {
		return false
	}
	if cert.Subject.String() != cert.Issuer.String() {
		return false
	}
	// CheckSignatureFrom refuses to treat cert as its own signer unless it
	// carries CA basic constraints, which most self-signed leaf certificates
	// (e.g. from openssl without -CA) don't set. CheckSignature verifies the
	// raw signature against the cert's own public key without that CA check.
	return cert.CheckSignature(cert.SignatureAlgorithm, cert.RawTBSCertificate, cert.Signature) == nil
}

// ScanURLs scans multiple targets concurrently.
func ScanURLs(ctx context.Context, targets []string, cfg *Config) []*CertInfo {
	if cfg == nil {
		cfg = DefaultConfig()
	}

	results := make([]*CertInfo, len(targets))
	sem := make(chan struct{}, cfg.Concurrency)
	var wg sync.WaitGroup

	for i, target := range targets {
		wg.Add(1)
		go func(idx int, t string) {
			defer wg.Done()
			sem <- struct{}{}
			defer func() { <-sem }()
			results[idx] = ScanURL(ctx, t, cfg)
		}(i, target)
	}

	wg.Wait()
	return results
}

// ScanCertificate evaluates a raw x509 certificate against thresholds.
func ScanCertificate(cert *x509.Certificate, source string, cfg *Config) *CertInfo {
	if cfg == nil {
		cfg = DefaultConfig()
	}
	info := &CertInfo{
		Host: source,
		Port: "-",
	}
	fillCertInfo(info, cert, cfg)
	return info
}

func fillCertInfo(info *CertInfo, cert *x509.Certificate, cfg *Config) {
	info.Subject = cert.Subject.CommonName
	info.Issuer = cert.Issuer.CommonName
	info.DNSNames = cert.DNSNames
	info.NotBefore = cert.NotBefore
	info.NotAfter = cert.NotAfter
	info.Serial = cert.SerialNumber.String()
	info.Fingerprint = fmt.Sprintf("%X", cert.SerialNumber.Bytes())

	now := time.Now()
	daysLeft := int(cert.NotAfter.Sub(now).Hours() / 24)
	info.DaysLeft = daysLeft

	switch {
	case now.After(cert.NotAfter):
		info.Status = StatusExpired
	case daysLeft <= cfg.CriticalThreshold:
		info.Status = StatusCritical
	case daysLeft <= cfg.WarnThreshold:
		info.Status = StatusWarning
	default:
		info.Status = StatusOK
	}
}

// parseTarget splits a target into host and port.
// Accepts: "example.com", "example.com:8443", "https://example.com"
func parseTarget(target string) (host, port string) {
	// Strip scheme
	stripped := target
	for _, scheme := range []string{"https://", "http://"} {
		if len(target) > len(scheme) && target[:len(scheme)] == scheme {
			stripped = target[len(scheme):]
			break
		}
	}
	// Strip path
	for i, ch := range stripped {
		if ch == '/' {
			stripped = stripped[:i]
			break
		}
	}

	if h, p, e := net.SplitHostPort(stripped); e == nil {
		return h, p
	}
	// Default to 443
	return stripped, "443"
}
