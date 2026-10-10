// Package revocation checks whether a TLS certificate has been revoked by
// its issuer, via OCSP (preferred) with a fallback to CRL distribution
// points.
package revocation

import (
	"bytes"
	"context"
	"crypto/x509"
	"fmt"
	"io"
	"math/big"
	"net/http"
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

// Check determines whether leaf has been revoked. It tries every OCSP
// responder URL in the certificate's AIA extension first, then falls back to
// every CRL distribution point, stopping at the first definitive answer.
// issuer is required to build and validate OCSP requests/responses; pass the
// leaf itself when it is self-signed. Check returns StatusUnknown with an
// explanatory Error when neither mechanism is available or reachable.
func Check(ctx context.Context, leaf, issuer *x509.Certificate, timeout time.Duration) Result {
	if leaf == nil {
		return Result{Status: StatusUnknown, Error: "no certificate to check"}
	}

	if issuer != nil {
		if res, ok := checkOCSP(ctx, leaf, issuer, timeout); ok {
			return res
		}
	}

	if res, ok := checkCRL(ctx, leaf, timeout); ok {
		return res
	}

	return Result{Status: StatusUnknown, Error: "no revocation information available"}
}

func checkOCSP(ctx context.Context, leaf, issuer *x509.Certificate, timeout time.Duration) (Result, bool) {
	if len(leaf.OCSPServer) == 0 {
		return Result{}, false
	}

	reqBytes, err := ocsp.CreateRequest(leaf, issuer, nil)
	if err != nil {
		return Result{}, false
	}

	client := &http.Client{Timeout: timeout}
	for _, server := range leaf.OCSPServer {
		if res, ok := queryOCSP(ctx, client, server, reqBytes, issuer); ok {
			return res, true
		}
	}
	return Result{}, false
}

func queryOCSP(ctx context.Context, client *http.Client, server string, reqBytes []byte, issuer *x509.Certificate) (Result, bool) {
	httpReq, err := http.NewRequestWithContext(ctx, http.MethodPost, server, bytes.NewReader(reqBytes))
	if err != nil {
		return Result{}, false
	}
	httpReq.Header.Set("Content-Type", "application/ocsp-request")
	httpReq.Header.Set("Accept", "application/ocsp-response")

	resp, err := client.Do(httpReq)
	if err != nil {
		return Result{}, false
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(io.LimitReader(resp.Body, 1<<20))
	if err != nil {
		return Result{}, false
	}

	ocspResp, err := ocsp.ParseResponse(body, issuer)
	if err != nil {
		return Result{}, false
	}

	switch ocspResp.Status {
	case ocsp.Good:
		return Result{Status: StatusGood, Method: "OCSP"}, true
	case ocsp.Revoked:
		return Result{
			Status: StatusRevoked,
			Method: "OCSP",
			Error:  fmt.Sprintf("revoked at %s", ocspResp.RevokedAt.Format(time.RFC3339)),
		}, true
	default:
		return Result{Status: StatusUnknown, Method: "OCSP", Error: "OCSP responder returned unknown status"}, true
	}
}

func checkCRL(ctx context.Context, leaf *x509.Certificate, timeout time.Duration) (Result, bool) {
	if len(leaf.CRLDistributionPoints) == 0 {
		return Result{}, false
	}

	client := &http.Client{Timeout: timeout}
	for _, url := range leaf.CRLDistributionPoints {
		if res, ok := queryCRL(ctx, client, url, leaf.SerialNumber); ok {
			return res, true
		}
	}
	return Result{}, false
}

func queryCRL(ctx context.Context, client *http.Client, url string, serial *big.Int) (Result, bool) {
	httpReq, err := http.NewRequestWithContext(ctx, http.MethodGet, url, http.NoBody)
	if err != nil {
		return Result{}, false
	}

	resp, err := client.Do(httpReq)
	if err != nil {
		return Result{}, false
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(io.LimitReader(resp.Body, 20<<20))
	if err != nil {
		return Result{}, false
	}

	crl, err := x509.ParseRevocationList(body)
	if err != nil {
		return Result{}, false
	}

	for _, revoked := range crl.RevokedCertificateEntries {
		if revoked.SerialNumber.Cmp(serial) == 0 {
			return Result{
				Status: StatusRevoked,
				Method: "CRL",
				Error:  fmt.Sprintf("revoked at %s", revoked.RevocationTime.Format(time.RFC3339)),
			}, true
		}
	}
	return Result{Status: StatusGood, Method: "CRL"}, true
}
