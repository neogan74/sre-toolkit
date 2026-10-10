package notifier

import (
	"context"
	"errors"
	"strings"
	"testing"
	"time"
)

type recorder struct {
	name string
	got  []Report
	err  error
}

func (r *recorder) Name() string {
	if r.name != "" {
		return r.name
	}
	return "rec"
}
func (r *recorder) Notify(_ context.Context, rep Report) error {
	if r.err != nil {
		return r.err
	}
	r.got = append(r.got, rep)
	return nil
}

func rep(hosts ...string) Report {
	r := Report{Summary: AlertSummary{Total: len(hosts) + 1}} // +1: a healthy cert
	for _, h := range hosts {
		r.Alerts = append(r.Alerts, CertAlert{Host: h, Status: "CRITICAL"})
	}
	return r
}

func newTestEsc(after time.Duration, t ...Notifier) (*Escalation, *time.Time) {
	clock := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)
	e := NewEscalation(after, t...)
	e.now = func() time.Time { return clock }
	return e, &clock
}

func TestEscalation_Lifecycle(t *testing.T) {
	rec := &recorder{}
	e, clock := newTestEsc(2*time.Hour, rec)
	ctx := context.Background()

	// first sighting + below threshold: nothing
	if err := e.Escalate(ctx, rep("a:443"), 0); err != nil || len(rec.got) != 0 {
		t.Fatalf("early escalate: err=%v sent=%d", err, len(rec.got))
	}
	*clock = clock.Add(time.Hour)
	_ = e.Escalate(ctx, rep("a:443"), 0)
	if len(rec.got) != 0 {
		t.Fatal("escalated before threshold")
	}

	// threshold reached: once, flagged, with age
	*clock = clock.Add(time.Hour)
	_ = e.Escalate(ctx, rep("a:443"), 0)
	if len(rec.got) != 1 || !rec.got[0].Escalated || rec.got[0].Alerts[0].UnresolvedFor != "2h0m0s" {
		t.Fatalf("want one escalated report w/ 2h age, got %+v", rec.got)
	}

	// no repeat within the same episode
	*clock = clock.Add(10 * time.Hour)
	_ = e.Escalate(ctx, rep("a:443"), 0)
	if len(rec.got) != 1 {
		t.Fatalf("repeated escalation: %d", len(rec.got))
	}

	// resolved -> reset; new episode escalates again after threshold
	_ = e.Escalate(ctx, rep(), 0)
	_ = e.Escalate(ctx, rep("a:443"), 0)
	*clock = clock.Add(2 * time.Hour)
	_ = e.Escalate(ctx, rep("a:443"), 0)
	if len(rec.got) != 2 {
		t.Fatalf("want re-escalation after reset, got %d", len(rec.got))
	}
}

func TestEscalation_PerHostIndependence(t *testing.T) {
	rec := &recorder{}
	e, clock := newTestEsc(time.Hour, rec)
	ctx := context.Background()
	_ = e.Escalate(ctx, rep("old:443"), 0)
	*clock = clock.Add(time.Hour)
	_ = e.Escalate(ctx, rep("old:443", "new:443"), 0)
	if len(rec.got) != 1 || len(rec.got[0].Alerts) != 1 || rec.got[0].Alerts[0].Host != "old:443" {
		t.Fatalf("only old host should escalate: %+v", rec.got)
	}
}

func TestEscalation_SendFailureRetried(t *testing.T) {
	rec := &recorder{err: errors.New("boom")}
	e, clock := newTestEsc(time.Hour, rec)
	ctx := context.Background()
	_ = e.Escalate(ctx, rep("a:443"), 0)
	*clock = clock.Add(time.Hour)
	if err := e.Escalate(ctx, rep("a:443"), 0); err == nil {
		t.Fatal("expected error")
	}
	rec.err = nil
	if err := e.Escalate(ctx, rep("a:443"), 0); err != nil || len(rec.got) != 1 {
		t.Fatalf("retry failed: err=%v sent=%d", err, len(rec.got))
	}
}

func TestEscalatedRendering(t *testing.T) {
	r := rep("a:443")
	r.Escalated = true
	r.Alerts[0].UnresolvedFor = "3h0m0s"
	r.Timestamp = time.Now()

	msg := NewSlackNotifier("http://x", "", 0).format(r)
	if !strings.Contains(msg.Text, "ESCALATED") || !strings.Contains(msg.Attachments[0].Text, "unresolved for 3h0m0s") {
		t.Errorf("slack: %+v", msg)
	}
	mail := string(buildMessage(EmailConfig{From: "a@b", To: []string{"c@d"}}, r))
	if !strings.Contains(mail, "Subject: [ESCALATED]") || !strings.Contains(mail, "unresolved for 3h0m0s") {
		t.Errorf("email: %s", mail)
	}
}

func TestEscalation_PartialSinkFailureNoDuplicates(t *testing.T) {
	ok := &recorder{name: "slack"}
	bad := &recorder{name: "email", err: errors.New("smtp down")}
	e, clock := newTestEsc(time.Hour, ok, bad)
	ctx := context.Background()
	_ = e.Escalate(ctx, rep("a:443"), 0)
	*clock = clock.Add(time.Hour)

	if err := e.Escalate(ctx, rep("a:443"), 0); err == nil {
		t.Fatal("expected email error")
	}
	*clock = clock.Add(time.Hour)
	_ = e.Escalate(ctx, rep("a:443"), 0)
	if len(ok.got) != 1 {
		t.Fatalf("slack re-sent after partial failure: %d", len(ok.got))
	}
	bad.err = nil
	if err := e.Escalate(ctx, rep("a:443"), 0); err != nil || len(bad.got) != 1 || len(ok.got) != 1 {
		t.Fatalf("email retry: err=%v email=%d slack=%d", err, len(bad.got), len(ok.got))
	}
}

func TestEscalation_EmptyScanKeepsEpisodes(t *testing.T) {
	rec := &recorder{}
	e, clock := newTestEsc(time.Hour, rec)
	ctx := context.Background()
	_ = e.Escalate(ctx, rep("a:443"), 0)
	*clock = clock.Add(30 * time.Minute)
	_ = e.Escalate(ctx, Report{}, 0) // no results at all (e.g. cancelled scan)
	*clock = clock.Add(30 * time.Minute)
	_ = e.Escalate(ctx, rep("a:443"), 0)
	if len(rec.got) != 1 {
		t.Fatalf("episode lost on empty scan: %d", len(rec.got))
	}
}

func TestEscalation_SummaryMatchesAlerts(t *testing.T) {
	rec := &recorder{}
	e, clock := newTestEsc(time.Hour, rec)
	r := rep("old:443")
	r.Summary.Critical = 5
	_ = e.Escalate(context.Background(), r, 0)
	*clock = clock.Add(time.Hour)
	_ = e.Escalate(context.Background(), r, 0)
	got := rec.got[0].Summary
	if got.Total != 1 || got.Critical != 1 {
		t.Fatalf("summary not recomputed: %+v", got)
	}
}
