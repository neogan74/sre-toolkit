package main

import (
	"testing"
	"time"

	"github.com/neogan/sre-toolkit/internal/cert-monitor/notifier"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNotifyOpts_Build(t *testing.T) {
	t.Setenv(envSlackWebhook, "")

	sinks, _, err := (&notifyOpts{minSeverity: "warning"}).build(time.Second)
	require.NoError(t, err)
	assert.Empty(t, sinks)

	_, _, err = (&notifyOpts{minSeverity: "bogus"}).build(time.Second)
	assert.Error(t, err)

	_, _, err = (&notifyOpts{minSeverity: "warning", smtpHost: "mail"}).build(time.Second)
	assert.ErrorContains(t, err, "email alerts")

	o := &notifyOpts{
		minSeverity: "critical", alertChain: true,
		webhookURL: "http://w", slackURL: "http://s",
		smtpHost: "mail", smtpPort: 25, smtpFrom: "a@x", smtpTo: []string{"b@x"}, smtpTLS: "none",
	}
	sinks, opts, err := o.build(time.Second)
	require.NoError(t, err)
	assert.Equal(t, "webhook,slack,email", sinks.Name())
	assert.Equal(t, notifier.SeverityCritical, opts.MinSeverity)
	assert.True(t, opts.ChainAlerts)
}

func TestNotifyOpts_SlackFromEnv(t *testing.T) {
	t.Setenv(envSlackWebhook, "http://hooks.slack.test/x")
	sinks, _, err := (&notifyOpts{minSeverity: "warning"}).build(time.Second)
	require.NoError(t, err)
	assert.Equal(t, "slack", sinks.Name())
}
