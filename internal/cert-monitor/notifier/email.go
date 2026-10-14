package notifier

import (
	"context"
	"fmt"

	pkgnotifier "github.com/neogan/sre-toolkit/pkg/notifier"
)

// SMTP TLS modes.
const (
	TLSNone     = pkgnotifier.TLSNone
	TLSStartTLS = pkgnotifier.TLSStartTLS
	TLSImplicit = pkgnotifier.TLSImplicit
)

// EmailConfig configures the SMTP notifier.
type EmailConfig = pkgnotifier.EmailConfig

// EmailNotifier sends alert reports over SMTP.
type EmailNotifier struct {
	cfg  EmailConfig
	send func(ctx context.Context, cfg EmailConfig, msg []byte) error
}

// NewEmailNotifier validates cfg and returns an EmailNotifier.
func NewEmailNotifier(cfg EmailConfig) (*EmailNotifier, error) {
	if err := cfg.Validate(); err != nil {
		return nil, err
	}
	return &EmailNotifier{cfg: cfg, send: pkgnotifier.SendSMTP}, nil
}

// Name implements Notifier.
func (n *EmailNotifier) Name() string { return "email" }

// Notify implements Notifier.
func (n *EmailNotifier) Notify(ctx context.Context, r Report) error {
	if len(r.Alerts) == 0 {
		return nil
	}
	return n.send(ctx, n.cfg, buildMessage(n.cfg, r))
}

func worstLabel(r Report) string {
	switch r.Worst() {
	case SeverityExpired:
		return "EXPIRED/ERROR"
	case SeverityCritical:
		return "CRITICAL"
	}
	return "WARNING"
}

func buildMessage(cfg EmailConfig, r Report) []byte {
	m := toMessage(r)
	m.Subject = fmt.Sprintf("[cert-monitor] %d certificate alert(s) (worst: %s)", len(r.Alerts), worstLabel(r))
	if r.Escalated {
		m.Subject = "[ESCALATED] " + m.Subject
	}
	m.Summary = fmt.Sprintf("Scanned %d certificates: %d warning, %d critical, %d expired, %d errors.",
		r.Summary.Total, r.Summary.Warning, r.Summary.Critical, r.Summary.Expired, r.Summary.Errors)
	return pkgnotifier.BuildEmail(cfg, m)
}
