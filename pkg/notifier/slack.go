package notifier

import (
	"context"
	"fmt"
	"net/http"
	"strings"
	"time"
)

// MaxSlackLines caps the lines in one message to stay under Slack's text limits.
const MaxSlackLines = 40

// SlackSender posts messages to a Slack incoming webhook.
type SlackSender struct {
	URL      string
	Channel  string // optional override (legacy webhooks)
	Username string // optional override
	client   *http.Client
}

// NewSlackSender creates a SlackSender; zero timeout means 10s.
func NewSlackSender(url, channel, username string, timeout time.Duration) *SlackSender {
	if timeout == 0 {
		timeout = defaultTimeout
	}
	return &SlackSender{URL: url, Channel: channel, Username: username, client: &http.Client{Timeout: timeout}}
}

// SlackAttachment is a Slack legacy attachment.
type SlackAttachment struct {
	Color    string   `json:"color"`
	Text     string   `json:"text"`
	Fallback string   `json:"fallback"`
	MrkdwnIn []string `json:"mrkdwn_in,omitempty"`
}

// SlackMessage is the webhook payload.
type SlackMessage struct {
	Channel     string            `json:"channel,omitempty"`
	Username    string            `json:"username,omitempty"`
	Text        string            `json:"text"`
	Attachments []SlackAttachment `json:"attachments"`
}

// Name implements Sender.
func (n *SlackSender) Name() string { return "slack" }

// Send implements Sender.
func (n *SlackSender) Send(ctx context.Context, m Message) error {
	if len(m.Lines) == 0 {
		return nil
	}
	return PostJSON(ctx, n.client, n.URL, n.Format(m))
}

// Format renders m as a Slack payload.
func (n *SlackSender) Format(m Message) SlackMessage {
	color := "warning"
	if m.Danger {
		color = "danger"
	}
	var b strings.Builder
	for i, l := range m.Lines {
		if i == MaxSlackLines {
			fmt.Fprintf(&b, "_…and %d more_\n", len(m.Lines)-MaxSlackLines)
			break
		}
		fmt.Fprintf(&b, "*%s* `%s`", EscapeSlack(l.Label), EscapeSlack(l.Target))
		if l.Detail != "" {
			fmt.Fprintf(&b, " — %s", EscapeSlack(l.Detail))
		}
		b.WriteByte('\n')
	}
	return SlackMessage{
		Channel:  n.Channel,
		Username: n.Username,
		Text:     m.Header,
		Attachments: []SlackAttachment{{
			Color:    color,
			Text:     b.String(),
			Fallback: m.Header,
			MrkdwnIn: []string{"text"},
		}},
	}
}

// EscapeSlack neutralises the control characters Slack interprets in text
// (&, <, >), so untrusted names cannot inject links or mentions.
func EscapeSlack(s string) string {
	return strings.NewReplacer("&", "&amp;", "<", "&lt;", ">", "&gt;").Replace(s)
}
