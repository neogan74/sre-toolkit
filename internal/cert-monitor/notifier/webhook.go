// Package notifier provides alerting integrations for cert-monitor.
package notifier

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"time"

	"github.com/neogan/sre-toolkit/internal/cert-monitor/scanner"
)

// WebhookPayload is the JSON body sent to a webhook endpoint.
type WebhookPayload struct {
	Timestamp string       `json:"timestamp"`
	Source    string       `json:"source"`
	Alerts    []CertAlert  `json:"alerts"`
	Summary   AlertSummary `json:"summary"`
}

// CertAlert represents a single certificate alert.
type CertAlert struct {
	Host     string `json:"host"`
	Subject  string `json:"subject"`
	Status   string `json:"status"`
	DaysLeft int    `json:"days_left"`
	Expires  string `json:"expires"`
	Error    string `json:"error,omitempty"`
}

// AlertSummary contains aggregated counts.
type AlertSummary struct {
	Total    int `json:"total"`
	Warning  int `json:"warning"`
	Critical int `json:"critical"`
	Expired  int `json:"expired"`
	Errors   int `json:"errors"`
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

// Notify sends certificate alerts for any non-OK results.
// Returns nil if there are no alerts to send.
func (n *WebhookNotifier) Notify(ctx context.Context, results []*scanner.CertInfo) error {
	var alerts []CertAlert
	summary := AlertSummary{}

	for _, info := range results {
		summary.Total++
		switch info.Status {
		case scanner.StatusOK:
			continue
		case scanner.StatusWarning:
			summary.Warning++
		case scanner.StatusCritical:
			summary.Critical++
		case scanner.StatusExpired:
			summary.Expired++
		case scanner.StatusError:
			summary.Errors++
		}

		expires := ""
		if !info.NotAfter.IsZero() {
			expires = info.NotAfter.Format("2006-01-02")
		}
		alerts = append(alerts, CertAlert{
			Host:     fmt.Sprintf("%s:%s", info.Host, info.Port),
			Subject:  info.Subject,
			Status:   string(info.Status),
			DaysLeft: info.DaysLeft,
			Expires:  expires,
			Error:    info.Error,
		})
	}

	if len(alerts) == 0 {
		return nil
	}

	payload := WebhookPayload{
		Timestamp: time.Now().UTC().Format(time.RFC3339),
		Source:    "cert-monitor",
		Alerts:    alerts,
		Summary:   summary,
	}

	return n.send(ctx, payload)
}

func (n *WebhookNotifier) send(ctx context.Context, payload WebhookPayload) error {
	body, err := json.Marshal(payload)
	if err != nil {
		return fmt.Errorf("marshalling payload: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, n.URL, bytes.NewReader(body))
	if err != nil {
		return fmt.Errorf("creating request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := n.client.Do(req)
	if err != nil {
		return fmt.Errorf("sending webhook: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return fmt.Errorf("webhook returned status %d", resp.StatusCode)
	}
	return nil
}
