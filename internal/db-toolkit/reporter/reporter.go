// Package reporter formats db-toolkit output for terminal and JSON.
package reporter

import (
	"encoding/json"
	"fmt"
	"io"
	"strings"
	"text/tabwriter"

	"github.com/neogan/sre-toolkit/internal/db-toolkit/analyzer"
	"github.com/neogan/sre-toolkit/internal/db-toolkit/backup"
	"github.com/neogan/sre-toolkit/internal/db-toolkit/health"
	"github.com/neogan/sre-toolkit/pkg/logging"
)

// Format is the output format.
type Format string

const (
	FormatTable Format = "table"
	FormatJSON  Format = "json"
)

// PrintHealthReport writes a health report to w.
func PrintHealthReport(w io.Writer, r *health.Report, format Format) {
	if format == FormatJSON {
		enc := json.NewEncoder(w)
		enc.SetIndent("", "  ")
		if err := enc.Encode(r); err != nil {
			logger := logging.GetLogger()
			logger.Error().Err(err).Msg("Failed to encode health report")
		}
		return
	}

	statusIcon := func(s health.Status) string {
		switch s {
		case health.StatusOK:
			return "[OK]"
		case health.StatusWarning:
			return "[WARN]"
		case health.StatusCritical:
			return "[CRIT]"
		default:
			return "[??]"
		}
	}

	fmt.Fprintf(w, "\nDatabase Health Report\n")
	fmt.Fprintf(w, "======================\n")
	fmt.Fprintf(w, "Type:      %s\n", r.DBType)
	fmt.Fprintf(w, "Host:      %s:%d\n", r.Host, r.Port)
	fmt.Fprintf(w, "Database:  %s\n", r.Database)
	fmt.Fprintf(w, "Connected: %v  (latency: %s)\n", r.Connected, r.Latency)
	fmt.Fprintf(w, "Overall:   %s %s\n\n", statusIcon(r.Overall), r.Overall)

	tw := tabwriter.NewWriter(w, 0, 0, 2, ' ', 0)
	fmt.Fprintln(tw, "CHECK\tSTATUS\tVALUE\tMESSAGE")
	fmt.Fprintln(tw, strings.Repeat("-", 60))
	for _, c := range r.Checks {
		fmt.Fprintf(tw, "%s\t%s %s\t%s\t%s\n", c.Name, statusIcon(c.Status), c.Status, c.Value, c.Message)
	}
	tw.Flush()
}

// PrintAnalysisReport writes a performance analysis report to w.
func PrintAnalysisReport(w io.Writer, r *analyzer.Report, format Format) {
	if format == FormatJSON {
		enc := json.NewEncoder(w)
		enc.SetIndent("", "  ")
		_ = enc.Encode(r)
		return
	}

	fmt.Fprintf(w, "\nDatabase Performance Report\n")
	fmt.Fprintf(w, "============================\n")
	fmt.Fprintf(w, "Type:       %s\n", r.DBType)
	fmt.Fprintf(w, "Database:   %s\n", r.Database)
	fmt.Fprintf(w, "Analyzed:   %s\n\n", r.AnalyzedAt.Format("2006-01-02 15:04:05"))

	if len(r.TopTables) > 0 {
		fmt.Fprintln(w, "Top Tables by Size:")
		tw := tabwriter.NewWriter(w, 0, 0, 2, ' ', 0)
		fmt.Fprintln(tw, "  SCHEMA\tTABLE\tROWS\tTOTAL\tINDEX\tDATA")
		for _, t := range r.TopTables {
			fmt.Fprintf(tw, "  %s\t%s\t%d\t%s\t%s\t%s\n",
				t.Schema, t.Table, t.Rows, t.TotalSize, t.IndexSize, t.TableSize)
		}
		tw.Flush()
		fmt.Fprintln(w)
	}

	if len(r.SlowQueries) > 0 {
		fmt.Fprintln(w, "Slow Queries (by mean execution time):")
		tw := tabwriter.NewWriter(w, 0, 0, 2, ' ', 0)
		fmt.Fprintln(tw, "  CALLS\tMEAN\tMAX\tTOTAL\tROWS\tQUERY")
		for _, q := range r.SlowQueries {
			query := q.Query
			if len(query) > 60 {
				query = query[:57] + "..."
			}
			fmt.Fprintf(tw, "  %d\t%s\t%s\t%s\t%d\t%s\n",
				q.Calls, q.MeanTime, q.MaxTime, q.TotalTime, q.Rows, query)
		}
		tw.Flush()
		fmt.Fprintln(w)
	}

	if len(r.UnusedIndexes) > 0 {
		fmt.Fprintln(w, "Unused Indexes (consider dropping):")
		tw := tabwriter.NewWriter(w, 0, 0, 2, ' ', 0)
		fmt.Fprintln(tw, "  SCHEMA\tTABLE\tINDEX\tSIZE\tSCANS")
		for _, idx := range r.UnusedIndexes {
			fmt.Fprintf(tw, "  %s\t%s\t%s\t%s\t%d\n",
				idx.Schema, idx.Table, idx.Index, idx.Size, idx.Scans)
		}
		tw.Flush()
		fmt.Fprintln(w)
	}

	if len(r.TopIndexes) > 0 {
		fmt.Fprintln(w, "Top Used Indexes:")
		tw := tabwriter.NewWriter(w, 0, 0, 2, ' ', 0)
		fmt.Fprintln(tw, "  SCHEMA\tTABLE\tINDEX\tSCANS\tSIZE")
		for _, idx := range r.TopIndexes {
			fmt.Fprintf(tw, "  %s\t%s\t%s\t%d\t%s\n",
				idx.Schema, idx.Table, idx.Index, idx.Scans, idx.Size)
		}
		tw.Flush()
		fmt.Fprintln(w)
	}
}

// PrintBackupResult writes a backup result to w.
func PrintBackupResult(w io.Writer, r *backup.Result, format Format) {
	if format == FormatJSON {
		enc := json.NewEncoder(w)
		enc.SetIndent("", "  ")
		_ = enc.Encode(r)
		return
	}

	fmt.Fprintf(w, "\nBackup Complete\n")
	fmt.Fprintf(w, "===============\n")
	fmt.Fprintf(w, "File:       %s\n", r.FilePath)
	fmt.Fprintf(w, "Size:       %s\n", humanSize(r.Size))
	fmt.Fprintf(w, "Duration:   %s\n", r.Duration)
	fmt.Fprintf(w, "Compressed: %v\n", r.Compressed)
}

func humanSize(bytes int64) string {
	const unit = 1024
	if bytes < unit {
		return fmt.Sprintf("%d B", bytes)
	}
	div, exp := int64(unit), 0
	for n := bytes / unit; n >= unit; n /= unit {
		div *= unit
		exp++
	}
	return fmt.Sprintf("%.1f %cB", float64(bytes)/float64(div), "KMGTPE"[exp])
}
