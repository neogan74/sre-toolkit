package notifier

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestSlackNotifier_Payload(t *testing.T) {
	var got slackMessage
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		b, _ := io.ReadAll(r.Body)
		_ = json.Unmarshal(b, &got)
	}))
	defer srv.Close()

	n := NewSlackNotifier(srv.URL, "#certs", time.Second)
	rep := BuildReport(sampleResults(), Options{})
	require.NoError(t, n.Notify(context.Background(), rep))

	assert.Equal(t, "#certs", got.Channel)
	require.Len(t, got.Attachments, 1)
	assert.Equal(t, "danger", got.Attachments[0].Color)
	assert.Contains(t, got.Attachments[0].Text, "`crit:443`")
	assert.Contains(t, got.Text, "5 certificate alert")
}

func TestSlackNotifier_WarningColorAndNoAlerts(t *testing.T) {
	calls := 0
	var got slackMessage
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		calls++
		b, _ := io.ReadAll(r.Body)
		_ = json.Unmarshal(b, &got)
	}))
	defer srv.Close()
	n := NewSlackNotifier(srv.URL, "", time.Second)

	require.NoError(t, n.Notify(context.Background(), Report{}))
	assert.Equal(t, 0, calls)

	rep := Report{Alerts: []CertAlert{{Host: "w:443", Status: "WARNING", DaysLeft: 5, Expires: "2026-11-01"}}}
	require.NoError(t, n.Notify(context.Background(), rep))
	assert.Equal(t, "warning", got.Attachments[0].Color)
}

func TestSlackNotifier_Truncates(t *testing.T) {
	n := NewSlackNotifier("http://x", "", time.Second)
	rep := Report{}
	for i := 0; i < maxSlackAlerts+5; i++ {
		rep.Alerts = append(rep.Alerts, CertAlert{Host: "h", Status: "WARNING"})
	}
	msg := n.format(rep)
	assert.Contains(t, msg.Attachments[0].Text, "and 5 more")
	assert.Equal(t, maxSlackAlerts+1, strings.Count(msg.Attachments[0].Text, "\n"))
}

func TestSlackNotifier_ErrorDoesNotLeakURL(t *testing.T) {
	n := NewSlackNotifier("http://127.0.0.1:1/secret-token", "", time.Second)
	err := n.Notify(context.Background(), Report{Alerts: []CertAlert{{Host: "h", Status: "WARNING"}}})
	require.Error(t, err)
	assert.NotContains(t, err.Error(), "secret-token")

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusForbidden)
	}))
	defer srv.Close()
	err = NewSlackNotifier(srv.URL, "", time.Second).Notify(context.Background(), Report{Alerts: []CertAlert{{Host: "h"}}})
	assert.ErrorContains(t, err, "403")
}
