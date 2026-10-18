package main

import (
	"context"
	"fmt"
	"os"
	"time"

	"github.com/rs/zerolog"
	"github.com/spf13/cobra"

	"github.com/neogan/sre-toolkit/internal/alert-analyzer/analyzer"
	"github.com/neogan/sre-toolkit/internal/alert-analyzer/notify"
	"github.com/neogan/sre-toolkit/pkg/notifier"
)

// Environment fallbacks for secrets, so they stay out of shell history.
const (
	envSlackWebhook = "SLACK_WEBHOOK_URL"
	envSMTPPassword = "SMTP_PASSWORD"
)

const notifyTimeout = 10 * time.Second

// notifyOpts holds the notification flags of the analyze command.
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
	notifyOn     string
}

func (o *notifyOpts) addFlags(cmd *cobra.Command) {
	f := cmd.Flags()
	f.StringVar(&o.webhookURL, "webhook", "", "Webhook URL to POST problematic rules to (JSON)")
	f.StringVar(&o.slackURL, "slack-webhook", "", "Slack incoming-webhook URL (or $"+envSlackWebhook+")")
	f.StringVar(&o.slackChannel, "slack-channel", "", "Slack channel override (legacy webhooks)")
	f.StringVar(&o.smtpHost, "smtp-host", "", "SMTP server host; enables email notifications")
	f.IntVar(&o.smtpPort, "smtp-port", 587, "SMTP server port")
	f.StringVar(&o.smtpUser, "smtp-user", "", "SMTP username (password via $"+envSMTPPassword+")")
	f.StringVar(&o.smtpFrom, "smtp-from", "", "Email sender address")
	f.StringSliceVar(&o.smtpTo, "smtp-to", nil, "Email recipient(s), comma-separated or repeated")
	f.StringVar(&o.smtpTLS, "smtp-tls", notifier.TLSStartTLS, "SMTP TLS mode: none, starttls, tls")
	f.StringVar(&o.notifyOn, "notify-on", analyzer.RecommendationPriorityHigh, "Minimum priority to notify on: low, medium, high, critical")
}

// build validates the flags and returns the configured sinks and priority
// threshold. Called before analysis so misconfiguration fails fast.
func (o *notifyOpts) build() (notifier.Multi, int, error) {
	minRank, err := analyzer.ParsePriority(o.notifyOn)
	if err != nil {
		return nil, 0, fmt.Errorf("--notify-on: %w", err)
	}

	var sinks notifier.Multi
	if o.webhookURL != "" {
		sinks = append(sinks, notifier.NewWebhookSender(o.webhookURL, notifyTimeout))
	}
	slackURL := o.slackURL
	if slackURL == "" {
		slackURL = os.Getenv(envSlackWebhook)
	}
	if slackURL != "" {
		sinks = append(sinks, notifier.NewSlackSender(slackURL, o.slackChannel, "alert-analyzer", notifyTimeout))
	}
	if o.smtpHost != "" {
		email, err := notifier.NewEmailSender(notifier.EmailConfig{
			Host:     o.smtpHost,
			Port:     o.smtpPort,
			Username: o.smtpUser,
			Password: os.Getenv(envSMTPPassword),
			From:     o.smtpFrom,
			To:       o.smtpTo,
			TLS:      o.smtpTLS,
			Timeout:  notifyTimeout,
		})
		if err != nil {
			return nil, 0, fmt.Errorf("email notifications: %w", err)
		}
		sinks = append(sinks, email)
	}
	return sinks, minRank, nil
}

// dispatch sends the problematic rules in result to sinks. Failures are
// logged, never fatal: the analysis itself already succeeded. A non-nil dedup
// (monitor mode) skips messages whose findings were already reported.
func dispatch(ctx context.Context, logger zerolog.Logger, sinks notifier.Multi, minRank int, result *analysisResult, dedup *notify.Dedup) {
	if len(sinks) == 0 {
		return
	}
	now := time.Now()
	msg, ok := notify.BuildMessage(result.stats, result.recommendations, result.flapping, minRank, now)
	if dedup != nil {
		// Due also forgets cleared findings, so call it even when msg is empty.
		if !dedup.Due(msg, now) {
			ok = false
		}
	}
	if !ok {
		logger.Info().Msg("No new problematic rules at or above --notify-on; nothing to send")
		return
	}
	if err := sinks.Send(ctx, msg); err != nil {
		logger.Error().Err(err).Msg("Notification failed")
		return
	}
	if dedup != nil {
		dedup.Commit(msg, now)
	}
	logger.Info().Str("sinks", sinks.Name()).Int("rules", len(msg.Lines)).Msg("Notifications sent")
}
