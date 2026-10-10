package notifier

import (
	"sync"
	"time"
)

type dedupEntry struct {
	status   string
	lastSent time.Time
}

// Dedup suppresses repeat alerts across successive scans (used by `watch`).
// An alert passes if it is new, its status changed, or Renotify has elapsed.
type Dedup struct {
	Renotify time.Duration
	now      func() time.Time

	mu   sync.Mutex
	seen map[string]dedupEntry
}

// NewDedup creates a Dedup; renotify <= 0 means never repeat an unchanged alert.
func NewDedup(renotify time.Duration) *Dedup {
	return &Dedup{Renotify: renotify, now: time.Now, seen: map[string]dedupEntry{}}
}

// Filter returns r restricted to alerts that should be sent now, and prunes
// state for certificates no longer alerting (so a recurrence notifies again).
func (d *Dedup) Filter(r Report) Report {
	d.mu.Lock()
	defer d.mu.Unlock()

	now := d.now()
	current := make(map[string]struct{}, len(r.Alerts))
	out := r
	out.Alerts = nil
	for _, a := range r.Alerts {
		current[a.Host] = struct{}{}
		e, ok := d.seen[a.Host]
		due := !ok || e.status != a.Status || (d.Renotify > 0 && now.Sub(e.lastSent) >= d.Renotify)
		if !due {
			continue
		}
		d.seen[a.Host] = dedupEntry{status: a.Status, lastSent: now}
		out.Alerts = append(out.Alerts, a)
	}
	for k := range d.seen {
		if _, ok := current[k]; !ok {
			delete(d.seen, k)
		}
	}
	return out
}
