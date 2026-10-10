package notifier

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/url"
)

// postJSON marshals v and POSTs it; any non-2xx status is an error.
func postJSON(ctx context.Context, client *http.Client, endpoint string, v any) error {
	body, err := json.Marshal(v)
	if err != nil {
		return fmt.Errorf("marshalling payload: %w", err)
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, endpoint, bytes.NewReader(body))
	if err != nil {
		return fmt.Errorf("creating request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := client.Do(req) //nolint:gosec // URL is user-configured, SSRF is acceptable
	if err != nil {
		// url.Error embeds the URL, which may be a secret (Slack); strip it.
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
