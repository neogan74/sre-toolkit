package k8ssecrets

import (
	"context"
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/pem"
	"math/big"
	"net"
	"testing"
	"time"

	"github.com/neogan/sre-toolkit/internal/cert-monitor/scanner"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes/fake"
)

func TestScanSecrets_Empty(t *testing.T) {
	client := fake.NewSimpleClientset()
	results, err := ScanSecrets(context.Background(), client, "", nil)
	require.NoError(t, err)
	assert.Empty(t, results)
}

func TestScanSecrets_ValidCert(t *testing.T) {
	certPEM, _ := generateCertPEM(t, time.Now().Add(-time.Hour), time.Now().Add(60*24*time.Hour))

	secret := &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "my-tls-secret",
			Namespace: "default",
		},
		Type: corev1.SecretTypeTLS,
		Data: map[string][]byte{
			"tls.crt": certPEM,
			"tls.key": []byte("key"),
		},
	}

	client := fake.NewSimpleClientset(secret)
	cfg := scanner.DefaultConfig()
	results, err := ScanSecrets(context.Background(), client, "default", cfg)

	require.NoError(t, err)
	require.Len(t, results, 1)
	assert.Equal(t, "default", results[0].Namespace)
	assert.Equal(t, "my-tls-secret", results[0].SecretName)
	assert.Equal(t, scanner.StatusOK, results[0].Status)
	assert.Empty(t, results[0].Error)
}

func TestScanSecrets_ExpiredCert(t *testing.T) {
	certPEM, _ := generateCertPEM(t, time.Now().Add(-48*time.Hour), time.Now().Add(-time.Hour))

	secret := &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "expired-cert",
			Namespace: "production",
		},
		Type: corev1.SecretTypeTLS,
		Data: map[string][]byte{
			"tls.crt": certPEM,
			"tls.key": []byte("key"),
		},
	}

	client := fake.NewSimpleClientset(secret)
	results, err := ScanSecrets(context.Background(), client, "", nil)

	require.NoError(t, err)
	require.Len(t, results, 1)
	assert.Equal(t, scanner.StatusExpired, results[0].Status)
}

func TestScanSecrets_CriticalCert(t *testing.T) {
	certPEM, _ := generateCertPEM(t, time.Now().Add(-time.Hour), time.Now().Add(3*24*time.Hour))

	secret := &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "critical-cert",
			Namespace: "default",
		},
		Type: corev1.SecretTypeTLS,
		Data: map[string][]byte{
			"tls.crt": certPEM,
		},
	}

	client := fake.NewSimpleClientset(secret)
	cfg := scanner.DefaultConfig()
	results, err := ScanSecrets(context.Background(), client, "default", cfg)

	require.NoError(t, err)
	require.Len(t, results, 1)
	assert.Equal(t, scanner.StatusCritical, results[0].Status)
}

func TestScanSecrets_InvalidCertData(t *testing.T) {
	secret := &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "bad-cert",
			Namespace: "default",
		},
		Type: corev1.SecretTypeTLS,
		Data: map[string][]byte{
			"tls.crt": []byte("not-a-valid-cert"),
		},
	}

	client := fake.NewSimpleClientset(secret)
	results, err := ScanSecrets(context.Background(), client, "default", nil)

	require.NoError(t, err)
	require.Len(t, results, 1)
	assert.Equal(t, scanner.StatusError, results[0].Status)
	assert.NotEmpty(t, results[0].Error)
}

func TestScanSecrets_MissingTLSCrt(t *testing.T) {
	secret := &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "no-crt",
			Namespace: "default",
		},
		Type: corev1.SecretTypeTLS,
		Data: map[string][]byte{
			"tls.key": []byte("key"),
		},
	}

	client := fake.NewSimpleClientset(secret)
	results, err := ScanSecrets(context.Background(), client, "default", nil)

	require.NoError(t, err)
	assert.Empty(t, results)
}

func TestScanSecrets_MultipleNamespaces(t *testing.T) {
	certPEM1, _ := generateCertPEM(t, time.Now().Add(-time.Hour), time.Now().Add(90*24*time.Hour))
	certPEM2, _ := generateCertPEM(t, time.Now().Add(-time.Hour), time.Now().Add(5*24*time.Hour))

	secrets := []*corev1.Secret{
		{
			ObjectMeta: metav1.ObjectMeta{Name: "cert1", Namespace: "ns1"},
			Type:       corev1.SecretTypeTLS,
			Data:       map[string][]byte{"tls.crt": certPEM1},
		},
		{
			ObjectMeta: metav1.ObjectMeta{Name: "cert2", Namespace: "ns2"},
			Type:       corev1.SecretTypeTLS,
			Data:       map[string][]byte{"tls.crt": certPEM2},
		},
	}

	objs := make([]interface{}, len(secrets))
	for i, s := range secrets {
		objs[i] = s
	}
	client := fake.NewSimpleClientset(secrets[0], secrets[1])

	// Scan all namespaces
	results, err := ScanSecrets(context.Background(), client, "", nil)
	require.NoError(t, err)
	assert.Len(t, results, 2)

	// Scan specific namespace
	results, err = ScanSecrets(context.Background(), client, "ns1", nil)
	require.NoError(t, err)
	assert.Len(t, results, 1)
	assert.Equal(t, "ns1", results[0].Namespace)
}

func TestParseCertificates(t *testing.T) {
	certPEM, _ := generateCertPEM(t, time.Now().Add(-time.Hour), time.Now().Add(90*24*time.Hour))

	certs, err := parseCertificates(certPEM)
	require.NoError(t, err)
	assert.Len(t, certs, 1)
}

func TestParseCertificates_Invalid(t *testing.T) {
	_, err := parseCertificates([]byte("invalid"))
	require.NoError(t, err) // no error, just empty result
}

// generateCertPEM creates a self-signed certificate PEM for testing.
func generateCertPEM(t *testing.T, notBefore, notAfter time.Time) ([]byte, []byte) {
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

	keyDER, err := x509.MarshalECPrivateKey(key)
	require.NoError(t, err)

	certPEM := pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: certDER})
	keyPEM := pem.EncodeToMemory(&pem.Block{Type: "EC PRIVATE KEY", Bytes: keyDER})

	return certPEM, keyPEM
}
