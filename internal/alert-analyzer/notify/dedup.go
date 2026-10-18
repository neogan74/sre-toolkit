package notify

import (
	"time"

	"github.com/neogan/sre-toolkit/pkg/notifier"
)

// Dedup suppresses findings already reported, so a long-running monitor does
// not repeat them every cycle. A line counts as due when it is new, or when
// Renotify has elapsed since it was last sent.
type Dedup struct {
	Renotify time.Duration // <= 0: never repeat an unchanged line
	seen     map[string]time.Time
}

// NewDedup creates a Dedup; renotify <= 0 means never repeat an unchanged line.
func NewDedup(renotify time.Duration) *Dedup {
	return &Dedup{Renotify: renotify, seen: map[string]time.Time{}}
}

func lineKey(l notifier.Line) string { return l.Label + "|" + l.Target }

// Due reports whether msg contains a line that should be sent: new, or last
// sent at least Renotify ago. Lines absent from msg are forgotten, so a
// finding that clears and later returns is reported again. The whole message
// is sent when due, so its headline counts stay accurate; call Commit after a
// successful send.
func (d *Dedup) Due(msg notifier.Message, now time.Time) bool {
	current := make(map[string]struct{}, len(msg.Lines))
	due := false
	for _, l := range msg.Lines {
		k := lineKey(l)
		current[k] = struct{}{}
		last, ok := d.seen[k]
		if !ok || (d.Renotify > 0 && now.Sub(last) >= d.Renotify) {
			due = true
		}
	}
	for k := range d.seen {
		if _, ok := current[k]; !ok {
			delete(d.seen, k)
		}
	}
	return due
}

// Commit records every line of msg as sent at now.
func (d *Dedup) Commit(msg notifier.Message, now time.Time) {
	for _, l := range msg.Lines {
		d.seen[lineKey(l)] = now
	}
}
