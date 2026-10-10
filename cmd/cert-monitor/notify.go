package main

import (
	"context"
	"fmt"
	"os"
	"time"

	"github.com/neogan/sre-toolkit/internal/cert-monitor/notifier"
	"github.com/neogan/sre-toolkit/internal/cert-monitor/scanner"
	"github.com/rs/zerolog"
	"github.com/spf13/cobra"
)

// Environment fallbacks for secrets, so they stay out of shell history.
const (
	envSlackWebhook  = "SLACK_WEBHOOK_URL"
	envSMTPPassword  = "SMTP_PASSWORD"
	envEscalateSlack = "ESCALATE_SLACK_WEBHOOK_URL"
)

// notifyOpts holds the alerting flags shared by scan, k8s and watch.
type notifyOpts struct {
	webhookURL   string
	slackURL     string
	slackChannel string
	smtpHost     string
	smtpPort     int
	smtpUser     string
	smtpFrom     string
	smtpTo       []string
	smtpTLS      string
	minSeverity  string
	alertChain   bool
}

func (o *notifyOpts) addFlags(cmd *cobra.Command) {
	f := cmd.Flags()
	f.StringVar(&o.webhookURL, "webhook", "", "Webhook URL to send alerts to")
	f.StringVar(&o.slackURL, "slack-webhook", "", "Slack incoming-webhook URL (or $"+envSlackWebhook+")")
	f.StringVar(&o.slackChannel, "slack-channel", "", "Slack channel override (legacy webhooks)")
	f.StringVar(&o.smtpHost, "smtp-host", "", "SMTP server host; enables email alerts")
	f.IntVar(&o.smtpPort, "smtp-port", 587, "SMTP server port")
	f.StringVar(&o.smtpUser, "smtp-user", "", "SMTP username (password via $"+envSMTPPassword+")")
	f.StringVar(&o.smtpFrom, "smtp-from", "", "Email sender address")
	f.StringSliceVar(&o.smtpTo, "smtp-to", nil, "Email recipient(s), comma-separated or repeated")
	f.StringVar(&o.smtpTLS, "smtp-tls", notifier.TLSStartTLS, "SMTP TLS mode: none, starttls, tls")
	f.StringVar(&o.minSeverity, "min-severity", "warning", "Minimum severity to alert on: warning, critical, expired")
	f.BoolVar(&o.alertChain, "alert-chain", false, "Also alert on certificate chain verification failures")
}

// build validates the flags and returns the configured sinks. It is called
// before scanning so misconfiguration fails fast.
func (o *notifyOpts) build(timeout time.Duration) (notifier.Multi, notifier.Options, error) {
	sev, err := notifier.ParseSeverity(o.minSeverity)
	if err != nil {
		return nil, notifier.Options{}, err
	}
	opts := notifier.Options{MinSeverity: sev, ChainAlerts: o.alertChain}

	var sinks notifier.Multi
	if o.webhookURL != "" {
		sinks = append(sinks, notifier.NewWebhookNotifier(o.webhookURL, timeout))
	}
	slackURL := o.slackURL
	if slackURL == "" {
		slackURL = os.Getenv(envSlackWebhook)
	}
	if slackURL != "" {
		sinks = append(sinks, notifier.NewSlackNotifier(slackURL, o.slackChannel, timeout))
	}
	if o.smtpHost != "" {
		email, err := o.newEmail(o.smtpTo, timeout)
		if err != nil {
			return nil, opts, fmt.Errorf("email alerts: %w", err)
		}
		sinks = append(sinks, email)
	}
	return sinks, opts, nil
}

func (o *notifyOpts) newEmail(to []string, timeout time.Duration) (*notifier.EmailNotifier, error) {
	return notifier.NewEmailNotifier(notifier.EmailConfig{
		Host:     o.smtpHost,
		Port:     o.smtpPort,
		Username: o.smtpUser,
		Password: os.Getenv(envSMTPPassword),
		From:     o.smtpFrom,
		To:       to,
		TLS:      o.smtpTLS,
		Timeout:  timeout,
	})
}

// escalateOpts holds the watch-only escalation flags.
type escalateOpts struct {
	after    time.Duration
	slackURL string
	smtpTo   []string
}

func (e *escalateOpts) addFlags(cmd *cobra.Command) {
	f := cmd.Flags()
	f.DurationVar(&e.after, "escalate-after", 0, "Escalate an alert unresolved for this long (0 = off); needs an escalation sink")
	f.StringVar(&e.slackURL, "escalate-slack-webhook", "", "Slack webhook for escalations (or $"+envEscalateSlack+")")
	f.StringSliceVar(&e.smtpTo, "escalate-smtp-to", nil, "Extra email recipient(s) for escalations (uses --smtp-* server settings)")
}

// build returns the escalation policy, or nil when --escalate-after is unset.
func (e *escalateOpts) build(o *notifyOpts, timeout time.Duration) (*notifier.Escalation, error) {
	if e.after < 0 {
		return nil, fmt.Errorf("--escalate-after must not be negative")
	}
	if e.after == 0 {
		return nil, nil
	}
	var sinks notifier.Multi
	slackURL := e.slackURL
	if slackURL == "" {
		slackURL = os.Getenv(envEscalateSlack)
	}
	if slackURL != "" {
		sinks = append(sinks, notifier.NewSlackNotifier(slackURL, o.slackChannel, timeout))
	}
	if len(e.smtpTo) > 0 {
		email, err := o.newEmail(e.smtpTo, timeout)
		if err != nil {
			return nil, fmt.Errorf("escalation email: %w", err)
		}
		sinks = append(sinks, email)
	}
	if len(sinks) == 0 {
		return nil, fmt.Errorf("--escalate-after needs --escalate-slack-webhook or --escalate-smtp-to")
	}
	return notifier.NewEscalation(e.after, sinks...), nil
}

// dispatch builds a report from results, escalates (esc may be nil) and sends
// it to all sinks (dedup may be nil). Failures are logged, never fatal.
func dispatch(ctx context.Context, logger zerolog.Logger, sinks notifier.Multi, opts notifier.Options,
	dedup *notifier.Dedup, esc *notifier.Escalation, results []*scanner.CertInfo) {
	if len(sinks) == 0 && esc == nil {
		return
	}
	report := notifier.BuildReport(results, opts)
	if esc != nil {
		if err := esc.Escalate(ctx, report, 0); err != nil {
			logger.Error().Err(err).Msg("Escalation failed")
		}
	}
	if len(sinks) == 0 {
		return
	}
	if dedup != nil {
		report = dedup.Filter(report)
	}
	if len(report.Alerts) == 0 {
		return
	}
	if err := sinks.Notify(ctx, report); err != nil {
		logger.Error().Err(err).Msg("Notification failed")
		return
	}
	logger.Info().Str("sinks", sinks.Name()).Int("alerts", len(report.Alerts)).Msg("Notifications sent")
}
