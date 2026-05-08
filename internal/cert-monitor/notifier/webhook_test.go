package notifier

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/neogan/sre-toolkit/internal/cert-monitor/scanner"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestWebhookNotifier_NoAlerts(t *testing.T) {
	called := false
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		called = true
	}))
	defer srv.Close()

	n := NewWebhookNotifier(srv.URL, 5*time.Second)
	results := []*scanner.CertInfo{
		{Status: scanner.StatusOK},
		{Status: scanner.StatusOK},
	}

	err := n.Notify(context.Background(), results)
	require.NoError(t, err)
	assert.False(t, called, "webhook should not be called when all certs are OK")
}

func TestWebhookNotifier_WithAlerts(t *testing.T) {
	var received WebhookPayload

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body, _ := io.ReadAll(r.Body)
		_ = json.Unmarshal(body, &received)
		w.WriteHeader(http.StatusOK)
	}))
	defer srv.Close()

	n := NewWebhookNotifier(srv.URL, 5*time.Second)
	results := []*scanner.CertInfo{
		{Host: "ok.example.com", Port: "443", Status: scanner.StatusOK},
		{
			Host:     "warn.example.com",
			Port:     "443",
			Subject:  "warn.example.com",
			Status:   scanner.StatusWarning,
			DaysLeft: 20,
			NotAfter: time.Now().Add(20 * 24 * time.Hour),
		},
		{
			Host:     "crit.example.com",
			Port:     "443",
			Subject:  "crit.example.com",
			Status:   scanner.StatusCritical,
			DaysLeft: 3,
			NotAfter: time.Now().Add(3 * 24 * time.Hour),
		},
		{
			Host:   "err.example.com",
			Port:   "443",
			Status: scanner.StatusError,
			Error:  "connection refused",
		},
	}

	err := n.Notify(context.Background(), results)
	require.NoError(t, err)

	assert.Equal(t, "cert-monitor", received.Source)
	assert.Len(t, received.Alerts, 3) // OK is excluded
	assert.Equal(t, 1, received.Summary.Warning)
	assert.Equal(t, 1, received.Summary.Critical)
	assert.Equal(t, 1, received.Summary.Errors)
	assert.Equal(t, 4, received.Summary.Total)
}

func TestWebhookNotifier_ServerError(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
	}))
	defer srv.Close()

	n := NewWebhookNotifier(srv.URL, 5*time.Second)
	results := []*scanner.CertInfo{
		{Status: scanner.StatusCritical, Host: "x.example.com", Port: "443"},
	}

	err := n.Notify(context.Background(), results)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "500")
}

func TestWebhookNotifier_InvalidURL(t *testing.T) {
	n := NewWebhookNotifier("http://127.0.0.1:1", 1*time.Second)
	results := []*scanner.CertInfo{
		{Status: scanner.StatusExpired, Host: "x.example.com", Port: "443"},
	}

	err := n.Notify(context.Background(), results)
	assert.Error(t, err)
}

func TestWebhookNotifier_ExpiredAlert(t *testing.T) {
	var received WebhookPayload

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body, _ := io.ReadAll(r.Body)
		_ = json.Unmarshal(body, &received)
		w.WriteHeader(http.StatusOK)
	}))
	defer srv.Close()

	n := NewWebhookNotifier(srv.URL, 5*time.Second)
	results := []*scanner.CertInfo{
		{
			Host:     "expired.example.com",
			Port:     "443",
			Status:   scanner.StatusExpired,
			DaysLeft: -5,
			NotAfter: time.Now().Add(-5 * 24 * time.Hour),
		},
	}

	err := n.Notify(context.Background(), results)
	require.NoError(t, err)
	assert.Equal(t, 1, received.Summary.Expired)
}
