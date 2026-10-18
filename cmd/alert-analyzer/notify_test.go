package main

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/rs/zerolog"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/neogan/sre-toolkit/internal/alert-analyzer/analyzer"
	"github.com/neogan/sre-toolkit/pkg/notifier"
)

func TestNotifyOpts_Build(t *testing.T) {
	t.Setenv(envSlackWebhook, "")

	sinks, rank, err := (&notifyOpts{notifyOn: "high"}).build()
	require.NoError(t, err)
	assert.Empty(t, sinks)
	assert.Equal(t, 3, rank)

	_, _, err = (&notifyOpts{notifyOn: "urgent"}).build()
	assert.ErrorContains(t, err, "--notify-on")

	_, _, err = (&notifyOpts{notifyOn: "low", smtpHost: "smtp"}).build()
	assert.ErrorContains(t, err, "email notifications")

	_, _, err = (&notifyOpts{notifyOn: "low", smtpHost: "smtp", smtpFrom: "a@x", smtpTo: []string{"b@x"}, smtpTLS: "bogus"}).build()
	assert.ErrorContains(t, err, "tls mode")

	sinks, _, err = (&notifyOpts{
		notifyOn: "low", webhookURL: "http://w", slackURL: "http://s",
		smtpHost: "smtp", smtpFrom: "a@x", smtpTo: []string{"b@x"}, smtpTLS: notifier.TLSStartTLS,
	}).build()
	require.NoError(t, err)
	assert.Equal(t, "webhook,slack,email", sinks.Name())
}

func TestNotifyOpts_SlackFromEnv(t *testing.T) {
	t.Setenv(envSlackWebhook, "http://hook")
	sinks, _, err := (&notifyOpts{}).build()
	require.NoError(t, err)
	assert.Equal(t, "slack", sinks.Name())
}

func TestDispatch_SendsSlackMessage(t *testing.T) {
	var got notifier.SlackMessage
	calls := 0
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		calls++
		b, _ := io.ReadAll(r.Body)
		_ = json.Unmarshal(b, &got)
	}))
	defer srv.Close()

	t.Setenv(envSlackWebhook, "")
	sinks, rank, err := (&notifyOpts{slackURL: srv.URL, notifyOn: "high"}).build()
	require.NoError(t, err)

	result := &analysisResult{
		recommendations: []analyzer.Recommendation{
			{Priority: "critical", Target: "HighErrorRate", Summary: "fires constantly", Action: "raise threshold"},
			{Priority: "low", Target: "Minor", Summary: "meh"},
		},
	}
	dispatch(context.Background(), zerolog.Nop(), sinks, rank, result, nil)

	require.Equal(t, 1, calls)
	assert.Contains(t, got.Text, "1 problematic alert rule(s)")
	assert.Equal(t, "alert-analyzer", got.Username)
	require.Len(t, got.Attachments, 1)
	assert.Equal(t, "danger", got.Attachments[0].Color)
	assert.Contains(t, got.Attachments[0].Text, "`HighErrorRate`")
	assert.NotContains(t, got.Attachments[0].Text, "Minor")
}

func TestDispatch_NothingToSendOrNoSinks(t *testing.T) {
	calls := 0
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { calls++ }))
	defer srv.Close()
	t.Setenv(envSlackWebhook, "")
	sinks, rank, err := (&notifyOpts{slackURL: srv.URL, notifyOn: "critical"}).build()
	require.NoError(t, err)

	low := &analysisResult{recommendations: []analyzer.Recommendation{{Priority: "low", Target: "A"}}}
	dispatch(context.Background(), zerolog.Nop(), sinks, rank, low, nil)
	dispatch(context.Background(), zerolog.Nop(), nil, rank, low, nil)
	assert.Zero(t, calls)
}

func TestDispatch_SendFailureIsNotFatal(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
	}))
	defer srv.Close()
	t.Setenv(envSlackWebhook, "")
	sinks, rank, err := (&notifyOpts{slackURL: srv.URL}).build()
	require.NoError(t, err)
	res := &analysisResult{recommendations: []analyzer.Recommendation{{Priority: "high", Target: "A"}}}
	assert.NotPanics(t, func() { dispatch(context.Background(), zerolog.Nop(), sinks, rank, res, nil) })
}

func TestRunAnalyze_InvalidNotifyFlagsFailBeforeAnalysis(t *testing.T) {
	err := runAnalyze(analysisOptions{
		prometheusURLs: []string{"http://127.0.0.1:1"},
		notify:         notifyOpts{notifyOn: "urgent"},
	})
	assert.ErrorContains(t, err, "--notify-on")
}

func TestRunMonitor_InvalidNotifyFlagsFailFast(t *testing.T) {
	err := runMonitor(analysisOptions{
		prometheusURLs: []string{"http://127.0.0.1:1"},
		notify:         notifyOpts{notifyOn: "urgent"},
	}, "1m", "127.0.0.1:0", "/metrics", time.Hour)
	assert.ErrorContains(t, err, "--notify-on")
}
