package revocation

import (
	"context"
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/x509"
	"crypto/x509/pkix"
	"math/big"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"golang.org/x/crypto/ocsp"
)

type testCA struct {
	cert *x509.Certificate
	key  *ecdsa.PrivateKey
}

func newTestCA(t *testing.T) *testCA {
	t.Helper()

	key, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	require.NoError(t, err)

	template := &x509.Certificate{
		SerialNumber:          big.NewInt(1),
		Subject:               pkix.Name{CommonName: "test-ca"},
		NotBefore:             time.Now().Add(-time.Hour),
		NotAfter:              time.Now().Add(24 * time.Hour),
		IsCA:                  true,
		BasicConstraintsValid: true,
		KeyUsage:              x509.KeyUsageCertSign | x509.KeyUsageCRLSign | x509.KeyUsageDigitalSignature,
	}

	der, err := x509.CreateCertificate(rand.Reader, template, template, &key.PublicKey, key)
	require.NoError(t, err)

	cert, err := x509.ParseCertificate(der)
	require.NoError(t, err)

	return &testCA{cert: cert, key: key}
}

func (ca *testCA) issueLeaf(t *testing.T, serial int64, ocspURL, crlURL string) *x509.Certificate {
	t.Helper()

	key, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	require.NoError(t, err)

	template := &x509.Certificate{
		SerialNumber: big.NewInt(serial),
		Subject:      pkix.Name{CommonName: "leaf.example.com"},
		NotBefore:    time.Now().Add(-time.Hour),
		NotAfter:     time.Now().Add(24 * time.Hour),
		DNSNames:     []string{"leaf.example.com"},
	}
	if ocspURL != "" {
		template.OCSPServer = []string{ocspURL}
	}
	if crlURL != "" {
		template.CRLDistributionPoints = []string{crlURL}
	}

	der, err := x509.CreateCertificate(rand.Reader, template, ca.cert, &key.PublicKey, ca.key)
	require.NoError(t, err)

	cert, err := x509.ParseCertificate(der)
	require.NoError(t, err)
	return cert
}

func (ca *testCA) ocspResponse(t *testing.T, leaf *x509.Certificate, status int) []byte {
	t.Helper()

	resp, err := ocsp.CreateResponse(ca.cert, ca.cert, ocsp.Response{
		SerialNumber: leaf.SerialNumber,
		Status:       status,
		ThisUpdate:   time.Now().Add(-time.Minute),
		NextUpdate:   time.Now().Add(time.Hour),
		RevokedAt:    time.Now().Add(-time.Hour),
	}, ca.key)
	require.NoError(t, err)
	return resp
}

func (ca *testCA) crl(t *testing.T, revoked ...*x509.Certificate) []byte {
	t.Helper()

	entries := make([]x509.RevocationListEntry, 0, len(revoked))
	for _, c := range revoked {
		entries = append(entries, x509.RevocationListEntry{
			SerialNumber:   c.SerialNumber,
			RevocationTime: time.Now().Add(-time.Hour),
		})
	}

	template := &x509.RevocationList{
		Number:                    big.NewInt(1),
		ThisUpdate:                time.Now().Add(-time.Minute),
		NextUpdate:                time.Now().Add(time.Hour),
		RevokedCertificateEntries: entries,
	}

	der, err := x509.CreateRevocationList(rand.Reader, template, ca.cert, ca.key)
	require.NoError(t, err)
	return der
}

func TestCheck_NilCertificate(t *testing.T) {
	res := Check(context.Background(), nil, nil, time.Second)
	assert.Equal(t, StatusUnknown, res.Status)
	assert.NotEmpty(t, res.Error)
}

func TestCheck_NoRevocationInfo(t *testing.T) {
	ca := newTestCA(t)
	leaf := ca.issueLeaf(t, 2, "", "")

	res := Check(context.Background(), leaf, ca.cert, time.Second)
	assert.Equal(t, StatusUnknown, res.Status)
	assert.Empty(t, res.Method)
}

func TestCheck_OCSPGood(t *testing.T) {
	ca := newTestCA(t)

	var leaf *x509.Certificate
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/ocsp-response")
		_, _ = w.Write(ca.ocspResponse(t, leaf, ocsp.Good))
	}))
	defer srv.Close()

	leaf = ca.issueLeaf(t, 3, srv.URL, "")

	res := Check(context.Background(), leaf, ca.cert, 5*time.Second)
	assert.Equal(t, StatusGood, res.Status)
	assert.Equal(t, "OCSP", res.Method)
}

func TestCheck_OCSPRevoked(t *testing.T) {
	ca := newTestCA(t)

	var leaf *x509.Certificate
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/ocsp-response")
		_, _ = w.Write(ca.ocspResponse(t, leaf, ocsp.Revoked))
	}))
	defer srv.Close()

	leaf = ca.issueLeaf(t, 4, srv.URL, "")

	res := Check(context.Background(), leaf, ca.cert, 5*time.Second)
	assert.Equal(t, StatusRevoked, res.Status)
	assert.Equal(t, "OCSP", res.Method)
	assert.Contains(t, res.Error, "revoked at")
}

func TestCheck_OCSPUnreachableFallsBackToCRL(t *testing.T) {
	ca := newTestCA(t)

	var leaf *x509.Certificate
	crlSrv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write(ca.crl(t))
	}))
	defer crlSrv.Close()

	// OCSP server is deliberately unreachable (closed immediately) so the
	// checker must fall back to CRL.
	deadOCSP := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {}))
	deadOCSP.Close()

	leaf = ca.issueLeaf(t, 5, deadOCSP.URL, crlSrv.URL)

	res := Check(context.Background(), leaf, ca.cert, 2*time.Second)
	assert.Equal(t, StatusGood, res.Status)
	assert.Equal(t, "CRL", res.Method)
}

func TestCheck_CRLRevoked(t *testing.T) {
	ca := newTestCA(t)

	var leaf *x509.Certificate
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write(ca.crl(t, leaf))
	}))
	defer srv.Close()

	leaf = ca.issueLeaf(t, 6, "", srv.URL)

	res := Check(context.Background(), leaf, ca.cert, 5*time.Second)
	assert.Equal(t, StatusRevoked, res.Status)
	assert.Equal(t, "CRL", res.Method)
	assert.Contains(t, res.Error, "revoked at")
}

func TestCheck_NoIssuerSkipsOCSP(t *testing.T) {
	ca := newTestCA(t)

	var leaf *x509.Certificate
	ocspHit := false
	ocspSrv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		ocspHit = true
		_, _ = w.Write(ca.ocspResponse(t, leaf, ocsp.Good))
	}))
	defer ocspSrv.Close()

	crlSrv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write(ca.crl(t))
	}))
	defer crlSrv.Close()

	leaf = ca.issueLeaf(t, 7, ocspSrv.URL, crlSrv.URL)

	res := Check(context.Background(), leaf, nil, 2*time.Second)
	assert.False(t, ocspHit, "OCSP should not be queried without an issuer certificate")
	assert.Equal(t, StatusGood, res.Status)
	assert.Equal(t, "CRL", res.Method)
}
