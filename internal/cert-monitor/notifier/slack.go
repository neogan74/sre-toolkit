package notifier

import (
	"context"
	"fmt"
	"strings"
	"time"

	pkgnotifier "github.com/neogan/sre-toolkit/pkg/notifier"
)

// maxSlackAlerts caps the lines in one message to stay under Slack's text limits.
const maxSlackAlerts = pkgnotifier.MaxSlackLines

type slackMessage = pkgnotifier.SlackMessage

// SlackNotifier posts alerts to a Slack incoming webhook.
type SlackNotifier struct {
	sender *pkgnotifier.SlackSender
}

// NewSlackNotifier creates a SlackNotifier.
func NewSlackNotifier(url, channel string, timeout time.Duration) *SlackNotifier {
	return &SlackNotifier{sender: pkgnotifier.NewSlackSender(url, channel, "cert-monitor", timeout)}
}

// Name implements Notifier.
func (n *SlackNotifier) Name() string { return n.sender.Name() }

// Notify implements Notifier.
func (n *SlackNotifier) Notify(ctx context.Context, r Report) error {
	return n.sender.Send(ctx, slackText(r))
}

func (n *SlackNotifier) format(r Report) slackMessage {
	return n.sender.Format(slackText(r))
}

// slackText is the neutral message for chat sinks.
func slackText(r Report) pkgnotifier.Message {
	m := toMessage(r)
	m.Header = fmt.Sprintf(":warning: cert-monitor: %d certificate alert(s) (critical %d, expired %d, errors %d, of %d scanned)",
		len(r.Alerts), r.Summary.Critical, r.Summary.Expired, r.Summary.Errors, r.Summary.Total)
	if r.Escalated {
		m.Header = ":rotating_light: ESCALATED " + strings.TrimPrefix(m.Header, ":warning: ")
	}
	return m
}

// toMessage converts a Report to the shared neutral message (no header/subject).
func toMessage(r Report) pkgnotifier.Message {
	m := pkgnotifier.Message{
		Source: r.Source,
		Time:   r.Timestamp,
		Danger: r.Worst() >= SeverityCritical,
		Lines:  make([]pkgnotifier.Line, 0, len(r.Alerts)),
	}
	for _, a := range r.Alerts {
		m.Lines = append(m.Lines, pkgnotifier.Line{Label: a.Status, Target: a.Host, Detail: alertDetail(a)})
	}
	return m
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
	if a.UnresolvedFor != "" {
		parts = append(parts, "unresolved for "+a.UnresolvedFor)
	}
	if len(parts) == 0 {
		return a.Subject
	}
	return strings.Join(parts, "; ")
}
