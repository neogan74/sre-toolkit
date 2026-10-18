package notifier

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/url"
	"time"
)

const defaultTimeout = 10 * time.Second

// PostJSON marshals v and POSTs it; any non-2xx status is an error. Errors
// never contain the endpoint URL (it may embed a secret).
func PostJSON(ctx context.Context, client *http.Client, endpoint string, v any) error {
	body, err := json.Marshal(v)
	if err != nil {
		return fmt.Errorf("marshalling payload: %w", err)
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, endpoint, bytes.NewReader(body))
	if err != nil {
		return fmt.Errorf("creating request: %s", unwrapURLError(err))
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := client.Do(req) //nolint:gosec // URL is user-configured, SSRF is acceptable
	if err != nil {
		return fmt.Errorf("sending request: %s", unwrapURLError(err))
	}
	defer resp.Body.Close()

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return fmt.Errorf("returned status %d", resp.StatusCode)
	}
	return nil
}

func unwrapURLError(err error) string {
	var ue *url.Error
	if errors.As(err, &ue) {
		return ue.Err.Error()
	}
	return err.Error()
}

// WebhookSender POSTs the Message as JSON to an HTTP endpoint.
type WebhookSender struct {
	URL    string
	client *http.Client
}

// NewWebhookSender creates a WebhookSender; zero timeout means 10s.
func NewWebhookSender(endpoint string, timeout time.Duration) *WebhookSender {
	if timeout == 0 {
		timeout = defaultTimeout
	}
	return &WebhookSender{URL: endpoint, client: &http.Client{Timeout: timeout}}
}

// Name implements Sender.
func (n *WebhookSender) Name() string { return "webhook" }

// Send implements Sender.
func (n *WebhookSender) Send(ctx context.Context, m Message) error {
	if len(m.Lines) == 0 {
		return nil
	}
	if m.Time.IsZero() {
		m.Time = time.Now().UTC()
	}
	return PostJSON(ctx, n.client, n.URL, m)
}
