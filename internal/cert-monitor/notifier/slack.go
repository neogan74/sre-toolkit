package notifier

import (
	"context"
	"fmt"
	"net/http"
	"strings"
	"time"
)

// maxSlackAlerts caps the lines in one message to stay under Slack's text limits.
const maxSlackAlerts = 40

// SlackNotifier posts alerts to a Slack incoming webhook.
type SlackNotifier struct {
	URL      string
	Channel  string // optional override (legacy webhooks)
	Username string // optional override
	client   *http.Client
}

// NewSlackNotifier creates a SlackNotifier.
func NewSlackNotifier(url, channel string, timeout time.Duration) *SlackNotifier {
	if timeout == 0 {
		timeout = 10 * time.Second
	}
	return &SlackNotifier{
		URL:      url,
		Channel:  channel,
		Username: "cert-monitor",
		client:   &http.Client{Timeout: timeout},
	}
}

type slackAttachment struct {
	Color    string   `json:"color"`
	Text     string   `json:"text"`
	Fallback string   `json:"fallback"`
	MrkdwnIn []string `json:"mrkdwn_in,omitempty"`
}

type slackMessage struct {
	Channel     string            `json:"channel,omitempty"`
	Username    string            `json:"username,omitempty"`
	Text        string            `json:"text"`
	Attachments []slackAttachment `json:"attachments"`
}

// Name implements Notifier.
func (n *SlackNotifier) Name() string { return "slack" }

// Notify implements Notifier.
func (n *SlackNotifier) Notify(ctx context.Context, r Report) error {
	if len(r.Alerts) == 0 {
		return nil
	}
	return postJSON(ctx, n.client, n.URL, n.format(r))
}

func (n *SlackNotifier) format(r Report) slackMessage {
	color := "warning"
	if r.Worst() >= SeverityCritical {
		color = "danger"
	}

	var b strings.Builder
	for i, a := range r.Alerts {
		if i == maxSlackAlerts {
			fmt.Fprintf(&b, "_…and %d more_\n", len(r.Alerts)-maxSlackAlerts)
			break
		}
		fmt.Fprintf(&b, "*%s* `%s` — %s\n", a.Status, a.Host, alertDetail(a))
	}

	header := fmt.Sprintf(":warning: cert-monitor: %d certificate alert(s) (critical %d, expired %d, errors %d, of %d scanned)",
		len(r.Alerts), r.Summary.Critical, r.Summary.Expired, r.Summary.Errors, r.Summary.Total)
	return slackMessage{
		Channel:  n.Channel,
		Username: n.Username,
		Text:     header,
		Attachments: []slackAttachment{{
			Color:    color,
			Text:     b.String(),
			Fallback: header,
			MrkdwnIn: []string{"text"},
		}},
	}
}

// alertDetail renders the human-readable tail of an alert line.
func alertDetail(a CertAlert) string {
	var parts []string
	if a.Expires != "" {
		parts = append(parts, fmt.Sprintf("expires %s (%d days)", a.Expires, a.DaysLeft))
	}
	if a.Error != "" {
		parts = append(parts, a.Error)
	}
	if a.ChainError != "" {
		parts = append(parts, "chain: "+a.ChainError)
	}
	if len(parts) == 0 {
		return a.Subject
	}
	return strings.Join(parts, "; ")
}
