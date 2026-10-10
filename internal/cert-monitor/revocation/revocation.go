// Package revocation checks whether a TLS certificate has been revoked by
// its issuer, via OCSP (preferred) with a fallback to CRL distribution
// points.
package revocation

import (
	"bytes"
	"context"
	"crypto/x509"
	"errors"
	"fmt"
	"io"
	"math/big"
	"net"
	"net/http"
	"net/netip"
	"sync"
	"syscall"
	"time"

	"golang.org/x/crypto/ocsp"
)

// Status represents the outcome of a revocation check.
type Status string

// Revocation status values.
const (
	StatusGood    Status = "GOOD"
	StatusRevoked Status = "REVOKED"
	StatusUnknown Status = "UNKNOWN"
)

// Result holds the outcome of checking a certificate's revocation status.
type Result struct {
	Status Status
	Method string // "OCSP", "CRL", or "" when no check was performed
	Error  string
}

const (
	maxOCSPResponse = 1 << 20
	maxCRLSize      = 20 << 20
	maxRedirects    = 3
	maxCRLCacheAge  = time.Hour
	clockSkew       = 5 * time.Minute
)

// allowLoopback disables the destination filter for loopback/link-local
// addresses. It exists only so tests can use httptest servers.
var allowLoopback = false

// FindIssuer returns the first candidate that actually signed leaf, or nil.
// Matching by signature avoids assuming the chain is presented in order. A
// self-signed leaf is returned as its own issuer.
func FindIssuer(leaf *x509.Certificate, candidates ...*x509.Certificate) *x509.Certificate {
	if leaf == nil {
		return nil
	}
	for _, c := range candidates {
		if c != nil && leaf.CheckSignatureFrom(c) == nil {
			return c
		}
	}
	// Self-signed leaf: it is its own issuer.
	if bytes.Equal(leaf.RawIssuer, leaf.RawSubject) &&
		leaf.CheckSignature(leaf.SignatureAlgorithm, leaf.RawTBSCertificate, leaf.Signature) == nil {
		return leaf
	}
	return nil
}

// Check determines whether leaf has been revoked. It tries every OCSP
// responder URL in the certificate's AIA extension first, then falls back to
// every CRL distribution point, stopping at the first definitive (GOOD or
// REVOKED) answer. An OCSP "unknown" answer is not definitive and does not
// prevent the CRL fallback. issuer is required both to build OCSP requests and
// to authenticate OCSP responses and CRLs; without it Check returns
// StatusUnknown rather than trusting unauthenticated data. Pass the leaf
// itself when it is self-signed.
//
// Revocation URLs are taken from the (untrusted) certificate, so requests are
// refused for loopback, link-local, unspecified and multicast destinations.
func Check(ctx context.Context, leaf, issuer *x509.Certificate, timeout time.Duration) Result {
	if leaf == nil {
		return Result{Status: StatusUnknown, Error: "no certificate to check"}
	}
	if issuer == nil {
		return Result{Status: StatusUnknown, Error: "issuer certificate unavailable; cannot authenticate revocation data"}
	}

	if timeout > 0 {
		var cancel context.CancelFunc
		ctx, cancel = context.WithTimeout(ctx, timeout)
		defer cancel()
	}
	client := newClient(timeout)

	ocspRes := checkOCSP(ctx, client, leaf, issuer)
	if isDefinitive(ocspRes) {
		return ocspRes
	}
	if crlRes := checkCRL(ctx, client, leaf, issuer); isDefinitive(crlRes) {
		return crlRes
	}
	if ocspRes.Status == StatusUnknown {
		return ocspRes
	}
	return Result{Status: StatusUnknown, Error: "no revocation information available"}
}

func isDefinitive(r Result) bool { return r.Status == StatusGood || r.Status == StatusRevoked }

// newClient builds an HTTP client that limits redirects and refuses to dial
// loopback/link-local/unspecified/multicast addresses (checked on the resolved
// IP, so DNS tricks and redirects cannot bypass it).
func newClient(timeout time.Duration) *http.Client {
	dialer := &net.Dialer{
		Timeout: timeout,
		Control: func(_, address string, _ syscall.RawConn) error {
			ap, err := netip.ParseAddrPort(address)
			if err != nil {
				return err
			}
			ip := ap.Addr().Unmap()
			if !allowLoopback && (ip.IsLoopback() || ip.IsLinkLocalUnicast() || ip.IsLinkLocalMulticast() ||
				ip.IsUnspecified() || ip.IsMulticast()) {
				return fmt.Errorf("refusing to contact %s", ip)
			}
			return nil
		},
	}
	return &http.Client{
		Timeout:   timeout,
		Transport: &http.Transport{DialContext: dialer.DialContext, DisableKeepAlives: true},
		CheckRedirect: func(_ *http.Request, via []*http.Request) error {
			if len(via) > maxRedirects {
				return errors.New("too many redirects")
			}
			return nil
		},
	}
}

// checkOCSP returns the first definitive answer, else the last "unknown"
// answer, else a zero Result.
func checkOCSP(ctx context.Context, client *http.Client, leaf, issuer *x509.Certificate) Result {
	if len(leaf.OCSPServer) == 0 {
		return Result{}
	}
	reqBytes, err := ocsp.CreateRequest(leaf, issuer, nil)
	if err != nil {
		return Result{}
	}

	var last Result
	for _, server := range leaf.OCSPServer {
		res := queryOCSP(ctx, client, server, reqBytes, issuer)
		if isDefinitive(res) {
			return res
		}
		if res.Status != "" {
			last = res
		}
	}
	return last
}

func queryOCSP(ctx context.Context, client *http.Client, server string, reqBytes []byte, issuer *x509.Certificate) Result {
	httpReq, err := http.NewRequestWithContext(ctx, http.MethodPost, server, bytes.NewReader(reqBytes))
	if err != nil {
		return Result{}
	}
	httpReq.Header.Set("Content-Type", "application/ocsp-request")
	httpReq.Header.Set("Accept", "application/ocsp-response")

	resp, err := client.Do(httpReq)
	if err != nil {
		return Result{}
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(io.LimitReader(resp.Body, maxOCSPResponse))
	if err != nil {
		return Result{}
	}

	// ParseResponse verifies the signature (issuer or its delegated responder).
	ocspResp, err := ocsp.ParseResponse(body, issuer)
	if err != nil {
		return Result{}
	}

	// Reject stale or not-yet-valid responses (replay protection).
	now := time.Now()
	if now.Add(clockSkew).Before(ocspResp.ThisUpdate) ||
		(!ocspResp.NextUpdate.IsZero() && now.After(ocspResp.NextUpdate)) {
		return Result{}
	}

	switch ocspResp.Status {
	case ocsp.Good:
		return Result{Status: StatusGood, Method: "OCSP"}
	case ocsp.Revoked:
		return Result{
			Status: StatusRevoked,
			Method: "OCSP",
			Error:  fmt.Sprintf("revoked at %s", ocspResp.RevokedAt.Format(time.RFC3339)),
		}
	default:
		return Result{Status: StatusUnknown, Method: "OCSP", Error: "OCSP responder returned unknown status"}
	}
}

func checkCRL(ctx context.Context, client *http.Client, leaf, issuer *x509.Certificate) Result {
	for _, url := range leaf.CRLDistributionPoints {
		if res := queryCRL(ctx, client, url, leaf.SerialNumber, issuer); isDefinitive(res) {
			return res
		}
	}
	return Result{}
}

func queryCRL(ctx context.Context, client *http.Client, url string, serial *big.Int, issuer *x509.Certificate) Result {
	crl := fetchCRL(ctx, client, url, issuer)
	if crl == nil {
		return Result{}
	}

	for _, revoked := range crl.RevokedCertificateEntries {
		if revoked.SerialNumber.Cmp(serial) == 0 {
			return Result{
				Status: StatusRevoked,
				Method: "CRL",
				Error:  fmt.Sprintf("revoked at %s", revoked.RevocationTime.Format(time.RFC3339)),
			}
		}
	}
	return Result{Status: StatusGood, Method: "CRL"}
}

type cachedCRL struct {
	crl     *x509.RevocationList
	expires time.Time
}

var (
	crlCacheMu sync.Mutex
	crlCache   = map[string]cachedCRL{}
)

func resetCRLCache() {
	crlCacheMu.Lock()
	defer crlCacheMu.Unlock()
	crlCache = map[string]cachedCRL{}
}

// fetchCRL downloads and parses a CRL, caching it until its NextUpdate (at
// most maxCRLCacheAge) so many certificates sharing a CA don't each trigger a
// full download. Stale CRLs (past NextUpdate) and CRLs not signed by issuer are
// rejected.
func fetchCRL(ctx context.Context, client *http.Client, url string, issuer *x509.Certificate) *x509.RevocationList {
	now := time.Now()
	crlCacheMu.Lock()
	c, ok := crlCache[url]
	crlCacheMu.Unlock()
	if ok && now.Before(c.expires) && c.crl.CheckSignatureFrom(issuer) == nil {
		return c.crl
	}

	httpReq, err := http.NewRequestWithContext(ctx, http.MethodGet, url, http.NoBody)
	if err != nil {
		return nil
	}
	resp, err := client.Do(httpReq)
	if err != nil {
		return nil
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(io.LimitReader(resp.Body, maxCRLSize))
	if err != nil {
		return nil
	}
	crl, err := x509.ParseRevocationList(body)
	if err != nil {
		return nil
	}
	if !crl.NextUpdate.IsZero() && now.After(crl.NextUpdate) {
		return nil
	}
	// The CRL arrives over an unauthenticated channel; trust it only if the
	// issuer signed it (also keeps forged CRLs out of the cache).
	if err := crl.CheckSignatureFrom(issuer); err != nil {
		return nil
	}

	expires := now.Add(maxCRLCacheAge)
	if !crl.NextUpdate.IsZero() && crl.NextUpdate.Before(expires) {
		expires = crl.NextUpdate
	}
	crlCacheMu.Lock()
	crlCache[url] = cachedCRL{crl: crl, expires: expires}
	crlCacheMu.Unlock()
	return crl
}
