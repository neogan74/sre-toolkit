// Package k8ssecrets provides scanning of Kubernetes TLS secrets.
package k8ssecrets

import (
	"context"
	"crypto/x509"
	"encoding/pem"
	"fmt"

	"github.com/neogan/sre-toolkit/internal/cert-monitor/scanner"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
)

// SecretCertInfo wraps CertInfo with Kubernetes metadata.
type SecretCertInfo struct {
	*scanner.CertInfo
	Namespace  string
	SecretName string
	KeyName    string
}

// ScanSecrets scans all TLS secrets in the given namespace (empty = all namespaces).
func ScanSecrets(ctx context.Context, client kubernetes.Interface, namespace string, cfg *scanner.Config) ([]*SecretCertInfo, error) {
	if cfg == nil {
		cfg = scanner.DefaultConfig()
	}

	secrets, err := client.CoreV1().Secrets(namespace).List(ctx, metav1.ListOptions{
		FieldSelector: "type=kubernetes.io/tls",
	})
	if err != nil {
		return nil, fmt.Errorf("listing TLS secrets: %w", err)
	}

	var results []*SecretCertInfo
	for i := range secrets.Items {
		s := &secrets.Items[i]
		infos := scanSecret(s, cfg)
		results = append(results, infos...)
	}
	return results, nil
}

func scanSecret(secret *corev1.Secret, cfg *scanner.Config) []*SecretCertInfo {
	var results []*SecretCertInfo

	certData, ok := secret.Data["tls.crt"]
	if !ok {
		return results
	}

	certs, err := parseCertificates(certData)
	if err != nil || len(certs) == 0 {
		results = append(results, &SecretCertInfo{
			CertInfo: &scanner.CertInfo{
				Host:   fmt.Sprintf("%s/%s", secret.Namespace, secret.Name),
				Status: scanner.StatusError,
				Error:  fmt.Sprintf("failed to parse certificate: %v", err),
			},
			Namespace:  secret.Namespace,
			SecretName: secret.Name,
			KeyName:    "tls.crt",
		})
		return results
	}

	// Report the leaf certificate (first one)
	source := fmt.Sprintf("%s/%s", secret.Namespace, secret.Name)
	info := scanner.ScanCertificate(certs[0], source, cfg)
	results = append(results, &SecretCertInfo{
		CertInfo:   info,
		Namespace:  secret.Namespace,
		SecretName: secret.Name,
		KeyName:    "tls.crt",
	})

	return results
}

func parseCertificates(data []byte) ([]*x509.Certificate, error) {
	var certs []*x509.Certificate
	for len(data) > 0 {
		var block *pem.Block
		block, data = pem.Decode(data)
		if block == nil {
			break
		}
		if block.Type != "CERTIFICATE" {
			continue
		}
		cert, err := x509.ParseCertificate(block.Bytes)
		if err != nil {
			return nil, err
		}
		certs = append(certs, cert)
	}
	return certs, nil
}
