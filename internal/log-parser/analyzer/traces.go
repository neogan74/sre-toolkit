package analyzer

import (
	"sort"
	"time"

	"github.com/neogan/sre-toolkit/internal/log-parser/formats"
)

const (
	// DefaultMaxTraces bounds how many distinct trace IDs are tracked, so a
	// huge log with unique IDs per line cannot exhaust memory.
	DefaultMaxTraces = 100_000
	// maxTraceSamples is how many entries are kept per trace.
	maxTraceSamples = 20
)

// TraceGroup is every line sharing one correlation ID (trace/request ID).
type TraceGroup struct {
	ID         string
	Entries    int
	Errors     int // ERROR and FATAL entries
	First      time.Time
	Last       time.Time
	Duration   time.Duration
	FirstError string // message of the first error, if any
	Samples    []*formats.Entry
}

// traceTracker groups entries by correlation ID.
type traceTracker struct {
	keys    []string
	max     int
	groups  map[string]*TraceGroup
	dropped int
}

func newTraceTracker(keys []string, limit int) *traceTracker {
	if limit <= 0 {
		limit = DefaultMaxTraces
	}
	return &traceTracker{keys: keys, max: limit, groups: map[string]*TraceGroup{}}
}

func (t *traceTracker) add(e *formats.Entry) {
	id := e.TraceID(t.keys)
	if id == "" {
		return
	}
	g, ok := t.groups[id]
	if !ok {
		if len(t.groups) >= t.max {
			t.dropped++
			return
		}
		g = &TraceGroup{ID: id}
		t.groups[id] = g
	}
	g.Entries++
	if !e.Timestamp.IsZero() {
		if g.First.IsZero() || e.Timestamp.Before(g.First) {
			g.First = e.Timestamp
		}
		if e.Timestamp.After(g.Last) {
			g.Last = e.Timestamp
		}
	}
	isErr := e.Level == formats.LevelError || e.Level == formats.LevelFatal
	if isErr {
		g.Errors++
		if g.FirstError == "" {
			g.FirstError = e.Message
		}
	}
	// Keep the first samples, but always make room for errors: they are why
	// the trace is interesting.
	switch {
	case len(g.Samples) < maxTraceSamples:
		g.Samples = append(g.Samples, e)
	case isErr:
		g.Samples[len(g.Samples)-1] = e
	}
}

// result returns the n traces with the most errors (ties: more entries, then
// ID), only those with at least one error. n <= 0 returns all of them.
func (t *traceTracker) result(n int) []TraceGroup {
	out := make([]TraceGroup, 0)
	for _, g := range t.groups {
		if g.Errors == 0 {
			continue
		}
		if !g.First.IsZero() {
			g.Duration = g.Last.Sub(g.First)
		}
		out = append(out, *g)
	}
	sort.Slice(out, func(i, j int) bool {
		if out[i].Errors != out[j].Errors {
			return out[i].Errors > out[j].Errors
		}
		if out[i].Entries != out[j].Entries {
			return out[i].Entries > out[j].Entries
		}
		return out[i].ID < out[j].ID
	})
	if n > 0 && len(out) > n {
		out = out[:n]
	}
	return out
}
