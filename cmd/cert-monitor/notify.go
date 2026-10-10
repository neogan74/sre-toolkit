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
	envSlackWebhook = "SLACK_WEBHOOK_URL"
	envSMTPPassword = "SMTP_PASSWORD"
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
		email, err := notifier.NewEmailNotifier(notifier.EmailConfig{
			Host:     o.smtpHost,
			Port:     o.smtpPort,
			Username: o.smtpUser,
			Password: os.Getenv(envSMTPPassword),
			From:     o.smtpFrom,
			To:       o.smtpTo,
			TLS:      o.smtpTLS,
			Timeout:  timeout,
		})
		if err != nil {
			return nil, opts, fmt.Errorf("email alerts: %w", err)
		}
		sinks = append(sinks, email)
	}
	return sinks, opts, nil
}

// dispatch builds a report from results and sends it to all sinks. dedup may
// be nil. Failures are logged, never fatal.
func dispatch(ctx context.Context, logger zerolog.Logger, sinks notifier.Multi, opts notifier.Options,
	dedup *notifier.Dedup, results []*scanner.CertInfo) {
	if len(sinks) == 0 {
		return
	}
	report := notifier.BuildReport(results, opts)
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
