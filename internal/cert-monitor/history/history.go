// Package history persists the last-seen certificate per scan target and a
// capped log of renewal events, in a single JSON file.
package history

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"sync"
	"time"

	"github.com/neogan/sre-toolkit/internal/cert-monitor/scanner"
)

// Event types.
const (
	EventFirstSeen = "first_seen"
	EventRenewed   = "renewed"
	EventChanged   = "changed" // new certificate whose NotAfter is not later
)

// MaxEventsPerTarget caps the retained events per target (oldest dropped).
const MaxEventsPerTarget = 50

// Event is one recorded certificate change.
type Event struct {
	Time        time.Time `json:"time"`
	Target      string    `json:"target"`
	Type        string    `json:"type"`
	OldSerial   string    `json:"old_serial,omitempty"`
	NewSerial   string    `json:"new_serial"`
	OldNotAfter time.Time `json:"old_not_after,omitzero"`
	NewNotAfter time.Time `json:"new_not_after"`
	Issuer      string    `json:"issuer,omitempty"`
}

// Target is the stored state for one scan target.
type Target struct {
	Serial      string    `json:"serial"`
	Fingerprint string    `json:"fingerprint"`
	Issuer      string    `json:"issuer"`
	NotAfter    time.Time `json:"not_after"`
	FirstSeen   time.Time `json:"first_seen"`
	LastSeen    time.Time `json:"last_seen"`
	Events      []Event   `json:"events"`
}

type file struct {
	Targets map[string]*Target `json:"targets"`
}

// Store is a JSON-file backed history. Safe for concurrent use within one
// process; concurrent processes on one file are last-writer-wins.
type Store struct {
	path string
	mu   sync.Mutex
	data file
}

// Open loads the history at path. A missing file yields an empty store; a
// corrupt file is an error (never silently overwritten).
func Open(path string) (*Store, error) {
	s := &Store{path: path, data: file{Targets: map[string]*Target{}}}
	b, err := os.ReadFile(path) //nolint:gosec // path comes from an operator-supplied flag
	if errors.Is(err, os.ErrNotExist) {
		return s, nil
	}
	if err != nil {
		return nil, fmt.Errorf("reading history: %w", err)
	}
	if err := json.Unmarshal(b, &s.data); err != nil {
		return nil, fmt.Errorf("history file %s is corrupt: %w", path, err)
	}
	if s.data.Targets == nil {
		s.data.Targets = map[string]*Target{}
	}
	return s, nil
}

// TargetKey is the history key for a scan result.
func TargetKey(c *scanner.CertInfo) string {
	if c.Port == "" {
		return c.Host
	}
	return c.Host + ":" + c.Port
}

// Record updates the history with results and saves it. It returns the new
// events. Results with an error or without a fingerprint are ignored so a
// failed scan never clobbers the last good certificate.
func (s *Store) Record(results []*scanner.CertInfo, now time.Time) ([]Event, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	var events []Event
	for _, c := range results {
		if c == nil || c.Error != "" || c.Fingerprint == "" {
			continue
		}
		key := TargetKey(c)
		t, ok := s.data.Targets[key]
		if !ok {
			t = &Target{FirstSeen: now}
			s.data.Targets[key] = t
			events = append(events, t.add(Event{Time: now, Target: key, Type: EventFirstSeen,
				NewSerial: c.Serial, NewNotAfter: c.NotAfter, Issuer: c.Issuer}))
		} else if t.Fingerprint != c.Fingerprint {
			typ := EventChanged
			if c.NotAfter.After(t.NotAfter) {
				typ = EventRenewed
			}
			events = append(events, t.add(Event{Time: now, Target: key, Type: typ,
				OldSerial: t.Serial, NewSerial: c.Serial,
				OldNotAfter: t.NotAfter, NewNotAfter: c.NotAfter, Issuer: c.Issuer}))
		}
		t.Serial, t.Fingerprint, t.Issuer, t.NotAfter, t.LastSeen = c.Serial, c.Fingerprint, c.Issuer, c.NotAfter, now
	}
	return events, s.save()
}

func (t *Target) add(e Event) Event {
	t.Events = append(t.Events, e)
	if n := len(t.Events); n > MaxEventsPerTarget {
		t.Events = append([]Event(nil), t.Events[n-MaxEventsPerTarget:]...)
	}
	return e
}

// Filter selects events. Zero values match everything.
type Filter struct {
	Target string
	Type   string
	Since  time.Time
}

// Events returns matching events, newest first.
func (s *Store) Events(f Filter) []Event {
	s.mu.Lock()
	defer s.mu.Unlock()

	var out []Event
	for key, t := range s.data.Targets {
		if f.Target != "" && f.Target != key {
			continue
		}
		for _, e := range t.Events {
			if (f.Type != "" && e.Type != f.Type) || e.Time.Before(f.Since) {
				continue
			}
			out = append(out, e)
		}
	}
	sort.SliceStable(out, func(i, j int) bool { return out[i].Time.After(out[j].Time) })
	return out
}

// TargetSummary is a per-target overview.
type TargetSummary struct {
	Target      string    `json:"target"`
	NotAfter    time.Time `json:"not_after"`
	LastSeen    time.Time `json:"last_seen"`
	Renewals    int       `json:"renewals"`
	LastRenewed time.Time `json:"last_renewed,omitzero"`
}

// Summary returns one row per target, sorted by target.
func (s *Store) Summary(target string) []TargetSummary {
	s.mu.Lock()
	defer s.mu.Unlock()

	out := make([]TargetSummary, 0, len(s.data.Targets))
	for key, t := range s.data.Targets {
		if target != "" && target != key {
			continue
		}
		row := TargetSummary{Target: key, NotAfter: t.NotAfter, LastSeen: t.LastSeen}
		for _, e := range t.Events {
			if e.Type == EventRenewed {
				row.Renewals++
				if e.Time.After(row.LastRenewed) {
					row.LastRenewed = e.Time
				}
			}
		}
		out = append(out, row)
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Target < out[j].Target })
	return out
}

func (s *Store) save() error {
	b, err := json.MarshalIndent(s.data, "", "  ")
	if err != nil {
		return fmt.Errorf("encoding history: %w", err)
	}
	dir := filepath.Dir(s.path)
	if err := os.MkdirAll(dir, 0o700); err != nil {
		return fmt.Errorf("creating history dir: %w", err)
	}
	tmp, err := os.CreateTemp(dir, ".history-*") // created 0600
	if err != nil {
		return fmt.Errorf("creating temp history: %w", err)
	}
	defer func() { _ = os.Remove(tmp.Name()) }() // no-op after successful rename
	_, werr := tmp.Write(b)
	cerr := tmp.Close()
	if err := errors.Join(werr, cerr); err != nil {
		return fmt.Errorf("writing history: %w", err)
	}
	if err := os.Rename(tmp.Name(), s.path); err != nil {
		return fmt.Errorf("replacing history: %w", err)
	}
	return nil
}
