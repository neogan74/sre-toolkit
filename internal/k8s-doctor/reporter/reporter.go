package reporter

import (
	"encoding/json"
	"fmt"
	"io"
	"os"
	"text/tabwriter"

	"github.com/neogan/sre-toolkit/internal/k8s-doctor/diagnostics"
	"github.com/neogan/sre-toolkit/internal/k8s-doctor/healthcheck"
)

// OutputFormat represents the output format
type OutputFormat string

const (
	FormatTable OutputFormat = "table"
	FormatJSON  OutputFormat = "json"
	FormatYAML  OutputFormat = "yaml"
)

// Reporter handles result reporting
type Reporter struct {
	format OutputFormat
	writer io.Writer
}

// NewReporter creates a new reporter
func NewReporter(format OutputFormat, writer io.Writer) *Reporter {
	if writer == nil {
		writer = os.Stdout
	}
	return &Reporter{
		format: format,
		writer: writer,
	}
}

// ReportNodeHealth reports node health status
func (r *Reporter) ReportNodeHealth(nodes []healthcheck.NodeStatus) error {
	switch r.format {
	case FormatJSON:
		return r.reportJSON(nodes)
	case FormatTable:
		return r.reportNodeTable(nodes)
	default:
		return fmt.Errorf("unsupported format: %s", r.format)
	}
}

// ReportPodHealth reports pod health status
func (r *Reporter) ReportPodHealth(status *healthcheck.PodStatus) error {
	switch r.format {
	case FormatJSON:
		return r.reportJSON(status)
	case FormatTable:
		return r.reportPodTable(status)
	default:
		return fmt.Errorf("unsupported format: %s", r.format)
	}
}

// ReportComponentHealth reports component health status
func (r *Reporter) ReportComponentHealth(components []healthcheck.ComponentStatus) error {
	switch r.format {
	case FormatJSON:
		return r.reportJSON(components)
	case FormatTable:
		return r.reportComponentTable(components)
	default:
		return fmt.Errorf("unsupported format: %s", r.format)
	}
}

// ReportDiagnostics reports diagnostics results
func (r *Reporter) ReportDiagnostics(result *diagnostics.DiagnosticsResult) error {
	switch r.format {
	case FormatJSON:
		return r.reportJSON(result)
	case FormatTable:
		return r.reportDiagnosticsTable(result)
	default:
		return fmt.Errorf("unsupported format: %s", r.format)
	}
}

// reportJSON outputs data as JSON
func (r *Reporter) reportJSON(data interface{}) error {
	encoder := json.NewEncoder(r.writer)
	encoder.SetIndent("", "  ")
	return encoder.Encode(data)
}

// reportNodeTable outputs node data as a table
func (r *Reporter) reportNodeTable(nodes []healthcheck.NodeStatus) error {
	w := tabwriter.NewWriter(r.writer, 0, 0, 2, ' ', 0)
	fmt.Fprintln(w, "NODE\tSTATUS\tROLES\tVERSION\tISSUES")
	fmt.Fprintln(w, "----\t------\t-----\t-------\t------")

	for _, node := range nodes {
		roles := ""
		for i, role := range node.Roles {
			if i > 0 {
				roles += ","
			}
			roles += role
		}

		issues := fmt.Sprintf("%d", len(node.Issues))
		if len(node.Issues) > 0 {
			issues += " ⚠"
		}

		// Add status indicator
		status := node.Status
		if node.Status == "Ready" {
			status = "✓ " + status
		} else {
			status = "✗ " + status
		}

		fmt.Fprintf(w, "%s\t%s\t%s\t%s\t%s\n",
			node.Name,
			status,
			roles,
			node.Version,
			issues,
		)
	}

	return w.Flush()
}

// reportPodTable outputs pod data as a table
func (r *Reporter) reportPodTable(status *healthcheck.PodStatus) error {
	fmt.Fprintf(r.writer, "\n=== Pod Summary ===\n")
	fmt.Fprintf(r.writer, "Total Pods:     %d\n", status.Total)
	fmt.Fprintf(r.writer, "Running:        %d\n", status.Running)
	fmt.Fprintf(r.writer, "Pending:        %d\n", status.Pending)
	fmt.Fprintf(r.writer, "Failed:         %d\n", status.Failed)
	fmt.Fprintf(r.writer, "Succeeded:      %d\n", status.Succeeded)
	fmt.Fprintf(r.writer, "Unknown:        %d\n", status.Unknown)
	fmt.Fprintf(r.writer, "\n")

	if len(status.ProblemPods) > 0 {
		fmt.Fprintf(r.writer, "=== Problem Pods (%d) ===\n", len(status.ProblemPods))
		w := tabwriter.NewWriter(r.writer, 0, 0, 2, ' ', 0)
		fmt.Fprintln(w, "NAMESPACE\tPOD\tSTATUS\tREASON\tRESTARTS")
		fmt.Fprintln(w, "---------\t---\t------\t------\t--------")

		for _, pod := range status.ProblemPods {
			fmt.Fprintf(w, "%s\t%s\t%s\t%s\t%d\n",
				pod.Namespace,
				pod.Name,
				pod.Status,
				pod.Reason,
				pod.Restarts,
			)
		}

		w.Flush()
	}

	return nil
}

// reportComponentTable outputs component data as a table
func (r *Reporter) reportComponentTable(components []healthcheck.ComponentStatus) error {
	w := tabwriter.NewWriter(r.writer, 0, 0, 2, ' ', 0)
	fmt.Fprintln(w, "COMPONENT\tSTATUS\tMESSAGE")
	fmt.Fprintln(w, "---------\t------\t-------")

	for _, comp := range components {
		status := comp.Status
		if comp.Status == "Healthy" {
			status = "✓ " + status
		} else {
			status = "✗ " + status
		}

		fmt.Fprintf(w, "%s\t%s\t%s\n",
			comp.Name,
			status,
			comp.Message,
		)
	}

	return w.Flush()
}

// reportDiagnosticsTable outputs diagnostics as a table
func (r *Reporter) reportDiagnosticsTable(result *diagnostics.DiagnosticsResult) error {
	// Summary
	fmt.Fprintf(r.writer, "\n=== Diagnostics Summary ===\n")
	fmt.Fprintf(r.writer, "Total Issues:   %d\n", result.Summary.TotalIssues)
	fmt.Fprintf(r.writer, "Critical:       %d\n", result.Summary.CriticalCount)
	fmt.Fprintf(r.writer, "Warning:        %d\n", result.Summary.WarningCount)
	fmt.Fprintf(r.writer, "Info:           %d\n", result.Summary.InfoCount)
	fmt.Fprintf(r.writer, "\n")

	// Node issues
	if len(result.NodeIssues) > 0 {
		fmt.Fprintf(r.writer, "=== Node Issues (%d) ===\n", len(result.NodeIssues))
		w := tabwriter.NewWriter(r.writer, 0, 0, 2, ' ', 0)
		fmt.Fprintln(w, "NODE\tSEVERITY\tTYPE\tMESSAGE")
		fmt.Fprintln(w, "----\t--------\t----\t-------")

		for _, issue := range result.NodeIssues {
			severity := issue.Severity
			if issue.Severity == "Critical" {
				severity = "🔴 " + severity
			} else if issue.Severity == "Warning" {
				severity = "⚠️  " + severity
			} else {
				severity = "ℹ️  " + severity
			}

			fmt.Fprintf(w, "%s\t%s\t%s\t%s\n",
				issue.Node,
				severity,
				issue.Type,
				issue.Message,
			)
		}
		w.Flush()
		fmt.Fprintln(r.writer)
	}

	// Pod issues
	if len(result.PodIssues) > 0 {
		fmt.Fprintf(r.writer, "=== Pod Issues (%d) ===\n", len(result.PodIssues))
		w := tabwriter.NewWriter(r.writer, 0, 0, 2, ' ', 0)
		fmt.Fprintln(w, "NAMESPACE\tPOD\tSEVERITY\tTYPE\tRESTARTS")
		fmt.Fprintln(w, "---------\t---\t--------\t----\t--------")

		for _, issue := range result.PodIssues {
			severity := issue.Severity
			if issue.Severity == "Critical" {
				severity = "🔴 " + severity
			} else if issue.Severity == "Warning" {
				severity = "⚠️  " + severity
			}

			fmt.Fprintf(w, "%s\t%s\t%s\t%s\t%d\n",
				issue.Namespace,
				issue.Pod,
				severity,
				issue.Type,
				issue.Restarts,
			)
		}
		w.Flush()
		fmt.Fprintln(r.writer)
	}

	// System issues
	if len(result.SystemIssues) > 0 {
		fmt.Fprintf(r.writer, "=== System Issues (%d) ===\n", len(result.SystemIssues))
		w := tabwriter.NewWriter(r.writer, 0, 0, 2, ' ', 0)
		fmt.Fprintln(w, "COMPONENT\tSEVERITY\tTYPE\tMESSAGE")
		fmt.Fprintln(w, "---------\t--------\t----\t-------")

		for _, issue := range result.SystemIssues {
			severity := "🔴 " + issue.Severity

			fmt.Fprintf(w, "%s\t%s\t%s\t%s\n",
				issue.Component,
				severity,
				issue.Type,
				issue.Message,
			)
		}
		w.Flush()
	}

	if result.Summary.TotalIssues == 0 {
		fmt.Fprintf(r.writer, "✓ No issues found! Cluster is healthy.\n")
	}

	return nil
}
