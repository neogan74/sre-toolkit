package notify

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"

	"github.com/neogan/sre-toolkit/pkg/notifier"
)

func msgOf(targets ...string) notifier.Message {
	m := notifier.Message{}
	for _, t := range targets {
		m.Lines = append(m.Lines, notifier.Line{Label: "HIGH", Target: t})
	}
	return m
}

func TestDedup(t *testing.T) {
	t0 := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)
	d := NewDedup(time.Hour)

	m := msgOf("A")
	assert.True(t, d.Due(m, t0), "new finding")
	d.Commit(m, t0)
	assert.False(t, d.Due(m, t0.Add(time.Minute)), "unchanged")

	m2 := msgOf("A", "B")
	assert.True(t, d.Due(m2, t0.Add(2*time.Minute)), "new line B")
	d.Commit(m2, t0.Add(2*time.Minute))

	assert.True(t, d.Due(m2, t0.Add(3*time.Hour)), "renotify elapsed")
}

func TestDedup_FailedSendIsRetried(t *testing.T) {
	d := NewDedup(time.Hour)
	m := msgOf("A")
	now := time.Now()
	assert.True(t, d.Due(m, now))
	assert.True(t, d.Due(m, now.Add(time.Minute)), "no Commit, still due")
}

func TestDedup_ClearedFindingIsReportedAgain(t *testing.T) {
	d := NewDedup(0)
	now := time.Now()
	m := msgOf("A")
	d.Commit(m, now)
	assert.False(t, d.Due(m, now.Add(time.Hour)), "renotify 0 never repeats")
	assert.False(t, d.Due(notifier.Message{}, now.Add(2*time.Hour)), "cleared")
	assert.True(t, d.Due(m, now.Add(3*time.Hour)), "returned")
}
