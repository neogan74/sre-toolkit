// Package reporter formats log-parser analysis results for output.
package reporter

import (
	"encoding/json"
	"fmt"
	"io"
	"strings"
	"text/tabwriter"

	"github.com/neogan/sre-toolkit/internal/log-parser/analyzer"
	"github.com/neogan/sre-toolkit/internal/log-parser/formats"
)

// Format represents output format.
type Format string

const (
	FormatTable Format = "table"
	FormatJSON  Format = "json"
)

// Reporter writes analysis results.
type Reporter struct {
	format Format
	out    io.Writer
}

// New creates a Reporter.
func New(format Format, out io.Writer) *Reporter {
	return &Reporter{format: format, out: out}
}

// Report outputs analysis stats.
func (r *Reporter) Report(stats *analyzer.Stats) error {
	if r.format == FormatJSON {
		enc := json.NewEncoder(r.out)
		enc.SetIndent("", "  ")
		return enc.Encode(stats)
	}
	return r.reportTable(stats)
}

// ReportMatches outputs matched entries from pattern search.
func (r *Reporter) ReportMatches(entries []*formats.Entry, pattern string) {
	fmt.Fprintf(r.out, "\nMatches for pattern %q:\n", pattern)
	fmt.Fprintln(r.out, strings.Repeat("-", 60))
	for _, e := range entries {
		ts := ""
		if !e.Timestamp.IsZero() {
			ts = e.Timestamp.Format("2006-01-02 15:04:05") + " "
		}
		fmt.Fprintf(r.out, "L%-5d %s[%s] %s\n", e.LineNum, ts, colorLevel(e.Level), e.Message)
	}
}

func (r *Reporter) reportTable(stats *analyzer.Stats) error {
	fmt.Fprintf(r.out, "\n%s Log Analysis Summary %s\n", strings.Repeat("=", 20), strings.Repeat("=", 20))

	// General stats
	fmt.Fprintf(r.out, "Total lines : %d\n", stats.TotalLines)
	fmt.Fprintf(r.out, "Parsed      : %d\n", stats.ParsedLines)
	fmt.Fprintf(r.out, "Parse errors: %d\n", stats.ErrorLines)
	if stats.RatePerMin > 0 {
		fmt.Fprintf(r.out, "Rate        : %.1f lines/min\n", stats.RatePerMin)
	}
	if !stats.TimeRange.Start.IsZero() {
		fmt.Fprintf(r.out, "Time range  : %s — %s\n",
			stats.TimeRange.Start.Format("2006-01-02 15:04:05"),
			stats.TimeRange.End.Format("2006-01-02 15:04:05"))
	}

	// Level breakdown
	fmt.Fprintf(r.out, "\n--- Level Breakdown ---\n")
	for _, lvl := range []formats.Level{
		formats.LevelFatal, formats.LevelError, formats.LevelWarning,
		formats.LevelInfo, formats.LevelDebug, formats.LevelTrace, formats.LevelUnknown,
	} {
		if c, ok := stats.LevelCounts[lvl]; ok && c > 0 {
			bar := makeBar(c, stats.ParsedLines, 30)
			fmt.Fprintf(r.out, "  %-8s %s %d\n", colorLevel(lvl), bar, c)
		}
	}

	// Top errors
	if len(stats.TopErrors) > 0 {
		fmt.Fprintf(r.out, "\n--- Top Errors ---\n")
		w := tabwriter.NewWriter(r.out, 0, 0, 2, ' ', 0)
		for i, mc := range stats.TopErrors {
			fmt.Fprintf(w, "  %2d. [%5d]  %s\n", i+1, mc.Count, truncate(mc.Message, 80))
		}
		_ = w.Flush()
	}

	// Top messages
	if len(stats.TopMessages) > 0 {
		fmt.Fprintf(r.out, "\n--- Top Messages ---\n")
		w := tabwriter.NewWriter(r.out, 0, 0, 2, ' ', 0)
		for i, mc := range stats.TopMessages {
			fmt.Fprintf(w, "  %2d. [%5d]  %s\n", i+1, mc.Count, truncate(mc.Message, 80))
		}
		_ = w.Flush()
	}

	// Pattern matches
	if len(stats.Patterns) > 0 {
		fmt.Fprintf(r.out, "\n--- Pattern Matches ---\n")
		for _, pm := range stats.Patterns {
			fmt.Fprintf(r.out, "  %-40s %d hits\n", pm.Pattern, pm.Count)
		}
	}

	// Anomalies
	if len(stats.Anomalies) > 0 {
		fmt.Fprintf(r.out, "\n--- Anomalies Detected ---\n")
		for _, a := range stats.Anomalies {
			ts := ""
			if !a.Time.IsZero() {
				ts = a.Time.Format("2006-01-02 15:04:05") + " "
			}
			fmt.Fprintf(r.out, "  %s%s (%d errors in window)\n", ts, truncate(a.Message, 60), a.Count)
		}
	}

	fmt.Fprintln(r.out)
	return nil
}

func makeBar(count, total, width int) string {
	if total == 0 {
		return strings.Repeat("░", width)
	}
	filled := count * width / total
	if filled > width {
		filled = width
	}
	return "\033[36m" + strings.Repeat("█", filled) + "\033[0m" + strings.Repeat("░", width-filled)
}

func colorLevel(lvl formats.Level) string {
	var color string
	switch lvl {
	case formats.LevelFatal:
		color = "\033[35m" // magenta
	case formats.LevelError:
		color = "\033[31m" // red
	case formats.LevelWarning:
		color = "\033[33m" // yellow
	case formats.LevelInfo:
		color = "\033[32m" // green
	case formats.LevelDebug:
		color = "\033[36m" // cyan
	default:
		color = "\033[90m" // grey
	}
	return color + string(lvl) + "\033[0m"
}

func truncate(s string, n int) string {
	if len(s) <= n {
		return s
	}
	return s[:n-3] + "..."
}
