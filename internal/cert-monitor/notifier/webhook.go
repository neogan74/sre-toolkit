// Package notifier provides alerting integrations for cert-monitor.
package notifier

import (
	"context"
	"net/http"
	"time"
)

// WebhookPayload is the JSON body sent to a webhook endpoint.
type WebhookPayload struct {
	Timestamp string       `json:"timestamp"`
	Source    string       `json:"source"`
	Alerts    []CertAlert  `json:"alerts"`
	Summary   AlertSummary `json:"summary"`
}

// WebhookNotifier sends alerts to an HTTP webhook endpoint.
type WebhookNotifier struct {
	URL     string
	Timeout time.Duration
	client  *http.Client
}

// NewWebhookNotifier creates a new WebhookNotifier.
func NewWebhookNotifier(url string, timeout time.Duration) *WebhookNotifier {
	if timeout == 0 {
		timeout = 10 * time.Second
	}
	return &WebhookNotifier{
		URL:     url,
		Timeout: timeout,
		client:  &http.Client{Timeout: timeout},
	}
}

// Name implements Notifier.
func (n *WebhookNotifier) Name() string { return "webhook" }

// Notify sends the report. Returns nil if there are no alerts.
func (n *WebhookNotifier) Notify(ctx context.Context, r Report) error {
	if len(r.Alerts) == 0 {
		return nil
	}
	return postJSON(ctx, n.client, n.URL, WebhookPayload{
		Timestamp: r.Timestamp.Format(time.RFC3339),
		Source:    r.Source,
		Alerts:    r.Alerts,
		Summary:   r.Summary,
	})
}
