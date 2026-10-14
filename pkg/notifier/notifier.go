// Package notifier delivers alert messages to Slack, email and HTTP webhooks.
// It is tool-agnostic: callers build a Message and fan it out to Senders.
package notifier

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"
)

// Line is one alert entry in a Message.
type Line struct {
	Label  string `json:"label"`            // short status, e.g. "CRITICAL"
	Target string `json:"target"`           // what it is about, e.g. host or rule name
	Detail string `json:"detail,omitempty"` // human-readable tail
}

// Message is a neutral, preformatted alert report.
type Message struct {
	Source  string    `json:"source"`  // sending tool, e.g. "cert-monitor"
	Time    time.Time `json:"time"`    // zero = now
	Header  string    `json:"header"`  // one-line chat headline
	Subject string    `json:"subject"` // email subject
	Summary string    `json:"summary,omitempty"`
	Lines   []Line    `json:"lines"`
	Danger  bool      `json:"danger"` // highest severity reached; drives Slack color
}

// Sender delivers a Message to one destination.
type Sender interface {
	// Name identifies the sink in logs (never include secrets).
	Name() string
	// Send delivers the message. Implementations are a no-op for messages without lines.
	Send(ctx context.Context, m Message) error
}

// Multi fans a message out to several senders. A failing sink does not stop
// the others; all errors are joined.
type Multi []Sender

// Name implements Sender.
func (m Multi) Name() string {
	names := make([]string, 0, len(m))
	for _, s := range m {
		names = append(names, s.Name())
	}
	return strings.Join(names, ",")
}

// Send implements Sender.
func (m Multi) Send(ctx context.Context, msg Message) error {
	var errs []error
	for _, s := range m {
		if err := s.Send(ctx, msg); err != nil {
			errs = append(errs, fmt.Errorf("%s: %w", s.Name(), err))
		}
	}
	return errors.Join(errs...)
}
