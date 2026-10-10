package notifier

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"
)

// Notifier delivers a certificate alert report to one destination.
type Notifier interface {
	// Name identifies the sink in logs (never include secrets).
	Name() string
	// Notify sends the report. Implementations must be a no-op for empty reports.
	Notify(ctx context.Context, r Report) error
}

// Escalator is an optional hook for escalation policies (e.g. re-page after
// an alert stays unresolved). No implementation ships yet.
type Escalator interface {
	Escalate(ctx context.Context, r Report, age time.Duration) error
}

// Multi fans a report out to several notifiers. A failing sink does not stop
// the others; all errors are joined.
type Multi []Notifier

// Name implements Notifier.
func (m Multi) Name() string {
	names := make([]string, 0, len(m))
	for _, n := range m {
		names = append(names, n.Name())
	}
	return strings.Join(names, ",")
}

// Notify implements Notifier.
func (m Multi) Notify(ctx context.Context, r Report) error {
	var errs []error
	for _, n := range m {
		if err := n.Notify(ctx, r); err != nil {
			errs = append(errs, fmt.Errorf("%s: %w", n.Name(), err))
		}
	}
	return errors.Join(errs...)
}
