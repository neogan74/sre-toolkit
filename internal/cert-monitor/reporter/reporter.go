// Package reporter provides output formatting for cert-monitor results.
package reporter

import (
	"encoding/json"
	"fmt"
	"io"
	"strings"
	"text/tabwriter"

	"github.com/neogan/sre-toolkit/internal/cert-monitor/scanner"
)

// Format represents the output format.
type Format string

const (
	FormatTable Format = "table"
	FormatJSON  Format = "json"
)

// Reporter writes certificate scan results to an output stream.
type Reporter struct {
	format Format
	out    io.Writer
}

// New creates a new Reporter.
func New(format Format, out io.Writer) *Reporter {
	return &Reporter{format: format, out: out}
}

// ReportURLScan writes URL scan results.
func (r *Reporter) ReportURLScan(results []*scanner.CertInfo) error {
	switch r.format {
	case FormatJSON:
		return r.writeJSON(results)
	default:
		return r.writeURLTable(results)
	}
}

// ReportSecretScan writes K8s secret scan results.
// It accepts []interface{} but callers should pass []*k8ssecrets.SecretCertInfo via the adapter.
func (r *Reporter) ReportCertList(results []CertRow) error {
	switch r.format {
	case FormatJSON:
		return r.writeJSON(results)
	default:
		return r.writeCertTable(results)
	}
}

// CertRow is a flat representation used for table/JSON output.
type CertRow struct {
	Source    string `json:"source"`
	Subject   string `json:"subject"`
	Issuer    string `json:"issuer"`
	NotAfter  string `json:"not_after"`
	DaysLeft  int    `json:"days_left"`
	Status    string `json:"status"`
	Error     string `json:"error,omitempty"`
	Namespace string `json:"namespace,omitempty"`
	Secret    string `json:"secret,omitempty"`
}

func (r *Reporter) writeURLTable(results []*scanner.CertInfo) error {
	w := tabwriter.NewWriter(r.out, 0, 0, 2, ' ', 0)
	fmt.Fprintln(w, "HOST\tPORT\tSUBJECT\tEXPIRES\tDAYS LEFT\tSTATUS")
	fmt.Fprintln(w, strings.Repeat("-", 80))
	for _, info := range results {
		expires := "-"
		daysLeft := "-"
		if !info.NotAfter.IsZero() {
			expires = info.NotAfter.Format("2006-01-02")
			daysLeft = fmt.Sprintf("%d", info.DaysLeft)
		}
		status := colorStatus(string(info.Status))
		errStr := ""
		if info.Error != "" {
			errStr = fmt.Sprintf(" (%s)", info.Error)
		}
		fmt.Fprintf(w, "%s\t%s\t%s\t%s\t%s\t%s%s\n",
			info.Host, info.Port, info.Subject, expires, daysLeft, status, errStr)
	}
	return w.Flush()
}

func (r *Reporter) writeCertTable(rows []CertRow) error {
	w := tabwriter.NewWriter(r.out, 0, 0, 2, ' ', 0)
	fmt.Fprintln(w, "SOURCE\tSUBJECT\tEXPIRES\tDAYS LEFT\tSTATUS")
	fmt.Fprintln(w, strings.Repeat("-", 80))
	for _, row := range rows {
		status := colorStatus(row.Status)
		errStr := ""
		if row.Error != "" {
			errStr = fmt.Sprintf(" (%s)", row.Error)
		}
		fmt.Fprintf(w, "%s\t%s\t%s\t%d\t%s%s\n",
			row.Source, row.Subject, row.NotAfter, row.DaysLeft, status, errStr)
	}
	return w.Flush()
}

func (r *Reporter) writeJSON(v interface{}) error {
	enc := json.NewEncoder(r.out)
	enc.SetIndent("", "  ")
	return enc.Encode(v)
}

// PrintSummary writes a summary line to the output.
func (r *Reporter) PrintSummary(total, ok, warn, critical, expired, errors int) {
	fmt.Fprintf(r.out, "\nSummary: %d checked | OK: %d | Warning: %d | Critical: %d | Expired: %d | Error: %d\n",
		total, ok, warn, critical, expired, errors)
}

func colorStatus(s string) string {
	switch scanner.Status(s) {
	case scanner.StatusOK:
		return "\033[32m" + s + "\033[0m"
	case scanner.StatusWarning:
		return "\033[33m" + s + "\033[0m"
	case scanner.StatusCritical:
		return "\033[31m" + s + "\033[0m"
	case scanner.StatusExpired:
		return "\033[35m" + s + "\033[0m"
	case scanner.StatusError:
		return "\033[90m" + s + "\033[0m"
	default:
		return s
	}
}
