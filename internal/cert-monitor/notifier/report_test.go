package notifier

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/neogan/sre-toolkit/internal/cert-monitor/scanner"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func sampleResults() []*scanner.CertInfo {
	return []*scanner.CertInfo{
		{Host: "ok", Port: "443", Status: scanner.StatusOK},
		{Host: "warn", Port: "443", Status: scanner.StatusWarning, DaysLeft: 20},
		{Host: "crit", Port: "443", Status: scanner.StatusCritical, DaysLeft: 3},
		{Host: "rev", Port: "443", Status: scanner.StatusRevoked},
		{Host: "exp", Port: "443", Status: scanner.StatusExpired, DaysLeft: -1},
		{Host: "err", Port: "443", Status: scanner.StatusError, Error: "refused"},
	}
}

func TestParseSeverity(t *testing.T) {
	for in, want := range map[string]Severity{"": SeverityWarning, "Warning": SeverityWarning, "critical": SeverityCritical, "expired": SeverityExpired} {
		got, err := ParseSeverity(in)
		require.NoError(t, err)
		assert.Equal(t, want, got)
	}
	_, err := ParseSeverity("nope")
	assert.Error(t, err)
}

func TestBuildReport_MinSeverity(t *testing.T) {
	r := BuildReport(sampleResults(), Options{})
	assert.Len(t, r.Alerts, 5)
	assert.Equal(t, 6, r.Summary.Total)

	r = BuildReport(sampleResults(), Options{MinSeverity: SeverityCritical})
	assert.Len(t, r.Alerts, 4) // crit, rev, exp, err
	assert.Equal(t, 0, r.Summary.Warning)

	r = BuildReport(sampleResults(), Options{MinSeverity: SeverityExpired})
	assert.Len(t, r.Alerts, 2) // exp, err (errors never filtered)
	assert.Equal(t, SeverityExpired, r.Worst())
}

func TestBuildReport_ChainAlerts(t *testing.T) {
	results := []*scanner.CertInfo{
		{Host: "a", Port: "443", Status: scanner.StatusOK, ChainError: "x509: unknown authority"},
		{Host: "b", Port: "443", Status: scanner.StatusOK}, // chain fine / unchecked
	}
	assert.Empty(t, BuildReport(results, Options{}).Alerts)

	r := BuildReport(results, Options{ChainAlerts: true})
	require.Len(t, r.Alerts, 1)
	assert.Equal(t, StatusChainInvalid, r.Alerts[0].Status)
	assert.Contains(t, r.Alerts[0].ChainError, "unknown authority")
}

type fakeNotifier struct {
	name  string
	err   error
	calls int
}

func (f *fakeNotifier) Name() string { return f.name }
func (f *fakeNotifier) Notify(context.Context, Report) error {
	f.calls++
	return f.err
}

func TestMulti_OneFailureDoesNotBlockOthers(t *testing.T) {
	bad := &fakeNotifier{name: "bad", err: errors.New("boom")}
	good := &fakeNotifier{name: "good"}
	m := Multi{bad, good}

	err := m.Notify(context.Background(), Report{})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "bad: boom")
	assert.Equal(t, 1, good.calls)
	assert.Equal(t, "bad,good", m.Name())
}

func TestDedup(t *testing.T) {
	now := time.Now()
	d := NewDedup(time.Hour)
	d.now = func() time.Time { return now }

	rep := func(status string) Report {
		return Report{Alerts: []CertAlert{{Host: "h:443", Status: status}}}
	}

	assert.Len(t, d.Filter(rep("WARNING")).Alerts, 1, "new alert passes")
	assert.Empty(t, d.Filter(rep("WARNING")).Alerts, "repeat suppressed")

	assert.Len(t, d.Filter(rep("CRITICAL")).Alerts, 1, "status change passes")

	now = now.Add(2 * time.Hour)
	assert.Len(t, d.Filter(rep("CRITICAL")).Alerts, 1, "renotify elapsed")

	assert.Empty(t, d.Filter(Report{}).Alerts) // recovered => state pruned
	assert.Len(t, d.Filter(rep("CRITICAL")).Alerts, 1, "recurrence notifies again")
}

func TestDedup_NoRenotify(t *testing.T) {
	now := time.Now()
	d := NewDedup(0)
	d.now = func() time.Time { return now }
	r := Report{Alerts: []CertAlert{{Host: "h", Status: "WARNING"}}}
	assert.Len(t, d.Filter(r).Alerts, 1)
	now = now.Add(1000 * time.Hour)
	assert.Empty(t, d.Filter(r).Alerts)
}
