package notifier

import (
	"fmt"
	"strings"
	"time"

	"github.com/neogan/sre-toolkit/internal/cert-monitor/scanner"
)

// StatusChainInvalid is the alert status for a certificate whose expiry is fine
// but whose chain failed verification.
const StatusChainInvalid = "CHAIN_INVALID"

// Severity is the minimum level an alert must reach to be sent.
type Severity int

// Severity levels, lowest first.
const (
	SeverityWarning Severity = iota + 1
	SeverityCritical
	SeverityExpired
)

// ParseSeverity parses warning|critical|expired (case-insensitive).
func ParseSeverity(s string) (Severity, error) {
	switch strings.ToLower(strings.TrimSpace(s)) {
	case "", "warning":
		return SeverityWarning, nil
	case "critical":
		return SeverityCritical, nil
	case "expired":
		return SeverityExpired, nil
	}
	return 0, fmt.Errorf("invalid min-severity %q (want warning, critical or expired)", s)
}

// CertAlert represents a single certificate alert.
type CertAlert struct {
	Host       string `json:"host"`
	Subject    string `json:"subject"`
	Status     string `json:"status"`
	DaysLeft   int    `json:"days_left"`
	Expires    string `json:"expires"`
	Error      string `json:"error,omitempty"`
	ChainError string `json:"chain_error,omitempty"`
}

// AlertSummary contains aggregated counts.
type AlertSummary struct {
	Total    int `json:"total"`
	Warning  int `json:"warning"`
	Critical int `json:"critical"`
	Expired  int `json:"expired"`
	Errors   int `json:"errors"`
}

// Report is the set of alerts produced by one scan.
type Report struct {
	Timestamp time.Time
	Source    string
	Alerts    []CertAlert
	Summary   AlertSummary
}

// Options controls which results become alerts.
type Options struct {
	MinSeverity Severity // zero value = SeverityWarning
	ChainAlerts bool     // also alert on chain verification failures
}

// BuildReport converts scan results into a Report. Summary.Total counts all
// results; only alerts at or above opts.MinSeverity are included. ERROR
// results are always included.
func BuildReport(results []*scanner.CertInfo, opts Options) Report {
	threshold := opts.MinSeverity
	if threshold == 0 {
		threshold = SeverityWarning
	}
	r := Report{Timestamp: time.Now().UTC(), Source: "cert-monitor"}
	r.Alerts = make([]CertAlert, 0, len(results))

	for _, info := range results {
		r.Summary.Total++
		status, sev, ok := classify(info, opts)
		if !ok || sev < threshold {
			continue
		}
		r.Summary.count(info.Status)

		expires := ""
		if !info.NotAfter.IsZero() {
			expires = info.NotAfter.Format("2006-01-02")
		}
		alert := CertAlert{
			Host:     fmt.Sprintf("%s:%s", info.Host, info.Port),
			Subject:  info.Subject,
			Status:   status,
			DaysLeft: info.DaysLeft,
			Expires:  expires,
			Error:    info.Error,
		}
		if opts.ChainAlerts {
			alert.ChainError = info.ChainError
		}
		r.Alerts = append(r.Alerts, alert)
	}
	return r
}

// classify maps a scan result to its alert status and severity. ok is false
// when the result needs no alert.
func classify(info *scanner.CertInfo, opts Options) (status string, sev Severity, ok bool) {
	status = string(info.Status)
	switch info.Status {
	case scanner.StatusOK:
		if opts.ChainAlerts && info.ChainError != "" {
			return StatusChainInvalid, SeverityWarning, true
		}
		return status, 0, false
	case scanner.StatusWarning:
		return status, SeverityWarning, true
	case scanner.StatusCritical, scanner.StatusRevoked:
		return status, SeverityCritical, true
	default: // EXPIRED, ERROR (errors are never filtered out)
		return status, SeverityExpired, true
	}
}

func (s *AlertSummary) count(st scanner.Status) {
	switch st {
	case scanner.StatusWarning:
		s.Warning++
	case scanner.StatusCritical, scanner.StatusRevoked:
		s.Critical++
	case scanner.StatusExpired:
		s.Expired++
	case scanner.StatusError:
		s.Errors++
	}
}

// Worst returns the highest severity among the alerts (0 if none).
func (r Report) Worst() Severity {
	var w Severity
	for _, a := range r.Alerts {
		if s := alertSeverity(a.Status); s > w {
			w = s
		}
	}
	return w
}

func alertSeverity(status string) Severity {
	switch scanner.Status(status) {
	case scanner.StatusCritical, scanner.StatusRevoked:
		return SeverityCritical
	case scanner.StatusExpired, scanner.StatusError:
		return SeverityExpired
	}
	return SeverityWarning
}
