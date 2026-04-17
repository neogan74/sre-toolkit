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

	host, port, err := parseTarget(target)
	if err != nil {
		return &CertInfo{
			Host:   target,
			Port:   "443",
			Status: StatusError,
			Error:  fmt.Sprintf("invalid target: %v", err),
		}
	}

	info := &CertInfo{
		Host: host,
		Port: port,
	}

	dialer := &tls.Dialer{
		NetDialer: &net.Dialer{
			Timeout: cfg.Timeout,
		},
		Config: &tls.Config{
			InsecureSkipVerify: cfg.InsecureSkipVerify, //nolint:gosec
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
	return info
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
func parseTarget(target string) (host, port string, err error) {
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
		return h, p, nil
	}
	// Default to 443
	return stripped, "443", nil
}
