package notifier

import (
	"context"
	"errors"
	"fmt"
	"sync"
	"time"

	"github.com/neogan/sre-toolkit/internal/cert-monitor/scanner"
)

type episode struct {
	first time.Time
	sent  map[string]bool // sink name -> escalation delivered
}

// Escalation re-alerts through separate Notifiers when an alert stays
// unresolved for After. Each sink gets one escalation per episode; an episode
// ends when the host stops alerting. State is in-memory only (lost on restart).
type Escalation struct {
	After   time.Duration
	Targets []Notifier
	now     func() time.Time

	mu  sync.Mutex
	eps map[string]*episode
}

var _ Escalator = (*Escalation)(nil)

// NewEscalation creates an Escalation sending to targets after the given age.
// Target names must be unique (they key per-sink delivery state).
func NewEscalation(after time.Duration, targets ...Notifier) *Escalation {
	return &Escalation{After: after, Targets: targets, now: time.Now, eps: map[string]*episode{}}
}

// Escalate implements Escalator. r must be the full (non-deduplicated) report
// of the latest scan; the age argument is unused because ages are tracked per
// host. A sink that fails is retried on the next call without re-sending to
// sinks that already succeeded.
func (e *Escalation) Escalate(ctx context.Context, r Report, _ time.Duration) error {
	pending := e.plan(r)

	var errs []error
	for i, t := range e.Targets {
		if len(pending[i]) == 0 {
			continue
		}
		out := r
		out.Alerts = pending[i]
		out.Escalated = true
		out.Summary = summarize(pending[i])
		if err := t.Notify(ctx, out); err != nil {
			errs = append(errs, fmt.Errorf("%s: %w", t.Name(), err))
			continue
		}
		e.markSent(t.Name(), pending[i])
	}
	return errors.Join(errs...)
}

// plan updates episodes from r and returns, per target, the alerts that are
// old enough and not yet delivered to it.
func (e *Escalation) plan(r Report) [][]CertAlert {
	now := e.now()
	e.mu.Lock()
	defer e.mu.Unlock()

	if r.Summary.Total > 0 { // an empty scan (e.g. cancelled) must not end episodes
		current := make(map[string]struct{}, len(r.Alerts))
		for _, a := range r.Alerts {
			current[a.Host] = struct{}{}
		}
		for k := range e.eps {
			if _, ok := current[k]; !ok {
				delete(e.eps, k)
			}
		}
	}
	for _, a := range r.Alerts {
		if _, ok := e.eps[a.Host]; !ok {
			e.eps[a.Host] = &episode{first: now, sent: map[string]bool{}}
		}
	}
	pending := make([][]CertAlert, len(e.Targets))
	for i, t := range e.Targets {
		for _, a := range r.Alerts {
			ep := e.eps[a.Host]
			age := now.Sub(ep.first)
			if ep.sent[t.Name()] || age < e.After {
				continue
			}
			a.UnresolvedFor = age.Round(time.Minute).String()
			pending[i] = append(pending[i], a)
		}
	}
	return pending
}

func (e *Escalation) markSent(sink string, alerts []CertAlert) {
	e.mu.Lock()
	defer e.mu.Unlock()
	for _, a := range alerts {
		if ep, ok := e.eps[a.Host]; ok {
			ep.sent[sink] = true
		}
	}
}

// summarize recounts severities for just the given alerts.
func summarize(alerts []CertAlert) AlertSummary {
	s := AlertSummary{Total: len(alerts)}
	for _, a := range alerts {
		s.count(scanner.Status(a.Status))
	}
	return s
}
