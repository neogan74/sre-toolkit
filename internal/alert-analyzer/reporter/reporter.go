// Package reporter provides functionality for reporting alert analysis results in different formats.
package reporter

import (
	"encoding/json"
	"fmt"
	"io"
	"text/tabwriter"
	"time"

	"github.com/neogan/sre-toolkit/internal/alert-analyzer/analyzer"
)

// Output format constants
const (
	FormatTable = "table"
	FormatJSON  = "json"
)

// Reporter handles output formatting for analysis results
type Reporter struct {
	format string
	writer io.Writer
}

// NewReporter creates a new reporter with the specified format
func NewReporter(format string, writer io.Writer) *Reporter {
	return &Reporter{
		format: format,
		writer: writer,
	}
}

// ReportSummary outputs the summary statistics
func (r *Reporter) ReportSummary(stats analyzer.SummaryStats) error {
	switch r.format {
	case FormatTable:
		return r.reportSummaryTable(stats)
	case FormatJSON:
		return r.reportSummaryJSON(stats)
	default:
		return fmt.Errorf("unsupported format: %s", r.format)
	}
}

// ReportFrequency outputs frequency analysis results
func (r *Reporter) ReportFrequency(results []analyzer.FrequencyResult) error {
	switch r.format {
	case FormatTable:
		return r.reportFrequencyTable(results)
	case FormatJSON:
		return r.reportFrequencyJSON(results)
	default:
		return fmt.Errorf("unsupported format: %s", r.format)
	}
}

// reportSummaryTable outputs summary in table format
func (r *Reporter) reportSummaryTable(stats analyzer.SummaryStats) error {
	fmt.Fprintln(r.writer, "\n=== Alert Analysis Summary ===")
	fmt.Fprintf(r.writer, "Total Alert Instances: %d\n", stats.TotalAlerts)
	fmt.Fprintf(r.writer, "Unique Alerts: %d\n", stats.UniqueAlerts)
	fmt.Fprintf(r.writer, "Total Firings: %d\n", stats.TotalFirings)
	fmt.Fprintf(r.writer, "Total Time Firing: %s\n", formatDuration(stats.TotalFiringTime))
	fmt.Fprintf(r.writer, "Average Duration: %s\n", formatDuration(stats.AvgDuration))
	fmt.Fprintf(r.writer, "Most Frequent Alert: %s\n", stats.MostFrequent)
	fmt.Fprintf(r.writer, "Longest Avg Duration: %s\n", stats.LongestAvgDuration)
	fmt.Fprintln(r.writer)

	return nil
}

// reportSummaryJSON outputs summary in JSON format
func (r *Reporter) reportSummaryJSON(stats analyzer.SummaryStats) error {
	encoder := json.NewEncoder(r.writer)
	encoder.SetIndent("", "  ")
	return encoder.Encode(map[string]interface{}{
		"summary": stats,
	})
}

// reportFrequencyTable outputs frequency analysis in table format
func (r *Reporter) reportFrequencyTable(results []analyzer.FrequencyResult) error {
	if len(results) == 0 {
		fmt.Fprintln(r.writer, "No alerts found in the analysis period.")
		return nil
	}

	w := tabwriter.NewWriter(r.writer, 0, 0, 3, ' ', 0)

	fmt.Fprintln(w, "=== Alert Frequency Analysis ===")
	fmt.Fprintln(w)
	fmt.Fprintln(w, "ALERT NAME\tFIRINGS\tAVG DURATION\tTOTAL TIME\tLAST FIRED\tSEVERITY")
	fmt.Fprintln(w, "----------\t-------\t------------\t----------\t----------\t--------")

	for _, result := range results {
		lastFired := result.LastFired.Format("2006-01-02 15:04")

		// Add severity indicator
		severityIcon := getSeverityIcon(result.Severity)

		fmt.Fprintf(w, "%s\t%d\t%s\t%s\t%s\t%s %s\n",
			result.AlertName,
			result.FiringCount,
			formatDuration(result.AvgDuration),
			formatDuration(result.TotalTime),
			lastFired,
			severityIcon,
			result.Severity,
		)
	}

	return w.Flush()
}

// reportFrequencyJSON outputs frequency analysis in JSON format
func (r *Reporter) reportFrequencyJSON(results []analyzer.FrequencyResult) error {
	encoder := json.NewEncoder(r.writer)
	encoder.SetIndent("", "  ")
	return encoder.Encode(map[string]interface{}{
		"frequency_analysis": results,
	})
}

// formatDuration formats a duration in a human-readable way
func formatDuration(d time.Duration) string {
	if d == 0 {
		return "0s"
	}

	// For durations less than 1 hour, show minutes and seconds
	if d < time.Hour {
		minutes := int(d.Minutes())
		seconds := int(d.Seconds()) % 60
		if minutes > 0 {
			return fmt.Sprintf("%dm %ds", minutes, seconds)
		}
		return fmt.Sprintf("%ds", seconds)
	}

	// For durations >= 1 hour, show hours and minutes
	hours := int(d.Hours())
	minutes := int(d.Minutes()) % 60
	if hours > 24 {
		days := hours / 24
		hours %= 24
		return fmt.Sprintf("%dd %dh %dm", days, hours, minutes)
	}
	return fmt.Sprintf("%dh %dm", hours, minutes)
}

// getSeverityIcon returns an emoji/icon for the severity level
func getSeverityIcon(severity string) string {
	switch severity {
	case "critical":
		return "🔴"
	case "warning":
		return "⚠️"
	case "info":
		return "ℹ️"
	default:
		return "❓"
	}
}

// AnalysisReport contains all analysis results for JSON export
type AnalysisReport struct {
	Timestamp string                     `json:"timestamp"`
	Summary   analyzer.SummaryStats      `json:"summary"`
	Frequency []analyzer.FrequencyResult `json:"frequency_analysis"`
	Flapping  []analyzer.FlappingResult  `json:"flapping_analysis,omitempty"`
}

// ReportComplete outputs a complete analysis report
func (r *Reporter) ReportComplete(stats analyzer.SummaryStats, frequency []analyzer.FrequencyResult) error {
	switch r.format {
	case FormatTable:
		if err := r.ReportSummary(stats); err != nil {
			return err
		}
		return r.ReportFrequency(frequency)
	case FormatJSON:
		report := AnalysisReport{
			Timestamp: time.Now().Format(time.RFC3339),
			Summary:   stats,
			Frequency: frequency,
		}
		encoder := json.NewEncoder(r.writer)
		encoder.SetIndent("", "  ")
		return encoder.Encode(report)
	default:
		return fmt.Errorf("unsupported format: %s", r.format)
	}
}

// ReportCompleteWithFlapping outputs a complete analysis report including flapping analysis
func (r *Reporter) ReportCompleteWithFlapping(stats analyzer.SummaryStats, frequency []analyzer.FrequencyResult, flapping []analyzer.FlappingResult) error {
	switch r.format {
	case FormatTable:
		if err := r.ReportSummary(stats); err != nil {
			return err
		}
		if err := r.ReportFrequency(frequency); err != nil {
			return err
		}
		return r.ReportFlapping(flapping)
	case FormatJSON:
		report := AnalysisReport{
			Timestamp: time.Now().Format(time.RFC3339),
			Summary:   stats,
			Frequency: frequency,
			Flapping:  flapping,
		}
		encoder := json.NewEncoder(r.writer)
		encoder.SetIndent("", "  ")
		return encoder.Encode(report)
	default:
		return fmt.Errorf("unsupported format: %s", r.format)
	}
}

// ReportFlapping outputs flapping analysis results
func (r *Reporter) ReportFlapping(results []analyzer.FlappingResult) error {
	switch r.format {
	case FormatTable:
		return r.reportFlappingTable(results)
	case FormatJSON:
		return r.reportFlappingJSON(results)
	default:
		return fmt.Errorf("unsupported format: %s", r.format)
	}
}

// reportFlappingTable outputs flapping analysis in table format
func (r *Reporter) reportFlappingTable(results []analyzer.FlappingResult) error {
	if len(results) == 0 {
		fmt.Fprintln(r.writer, "\nNo flapping alerts detected.")
		return nil
	}

	w := tabwriter.NewWriter(r.writer, 0, 0, 3, ' ', 0)

	fmt.Fprintln(w, "\n=== Flapping Alerts Analysis ===")
	fmt.Fprintln(w)
	fmt.Fprintln(w, "ALERT NAME\tTRANSITIONS\tFLAP SCORE\tAVG STATE DUR\tSHORTEST\tFLAPPING\tSEVERITY")
	fmt.Fprintln(w, "----------\t-----------\t----------\t-------------\t--------\t--------\t--------")

	for _, result := range results {
		flappingStatus := "No"
		if result.IsFlapping {
			flappingStatus = "🔄 Yes"
		}

		severityIcon := getSeverityIcon(result.Severity)

		fmt.Fprintf(w, "%s\t%d\t%.2f/hr\t%s\t%s\t%s\t%s %s\n",
			result.AlertName,
			result.TransitionCount,
			result.FlappingScore,
			formatDuration(result.AvgStateDuration),
			formatDuration(result.ShortestDuration),
			flappingStatus,
			severityIcon,
			result.Severity,
		)
	}

	return w.Flush()
}

// reportFlappingJSON outputs flapping analysis in JSON format
func (r *Reporter) reportFlappingJSON(results []analyzer.FlappingResult) error {
	encoder := json.NewEncoder(r.writer)
	encoder.SetIndent("", "  ")
	return encoder.Encode(map[string]interface{}{
		"flapping_analysis": results,
	})
}
