package main

import (
	"encoding/json"
	"fmt"
	"io"
	"os"
	"text/tabwriter"
	"time"

	"github.com/neogan/sre-toolkit/internal/cert-monitor/history"
	"github.com/neogan/sre-toolkit/internal/cert-monitor/scanner"
	"github.com/rs/zerolog"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"
)

const envHistoryFile = "CERT_MONITOR_HISTORY"

func addHistoryFlag(cmd *cobra.Command, path *string) {
	cmd.Flags().StringVar(path, "history-file", "", "Record certificate renewals in this JSON file (or $"+envHistoryFile+")")
}

func resolveHistoryPath(flag string) string {
	if flag != "" {
		return flag
	}
	return os.Getenv(envHistoryFile)
}

// recordHistory records results when a history file is configured. Failures
// are logged, never fatal.
func recordHistory(logger zerolog.Logger, flag string, results []*scanner.CertInfo) {
	path := resolveHistoryPath(flag)
	if path == "" {
		return
	}
	store, err := history.Open(path)
	if err != nil {
		logger.Warn().Err(err).Msg("History disabled for this scan")
		return
	}
	events, err := store.Record(results, time.Now().UTC())
	if err != nil {
		logger.Warn().Err(err).Msg("Failed to save history")
	}
	for _, e := range events {
		if e.Type == history.EventFirstSeen {
			continue
		}
		logger.Info().Str("target", e.Target).Str("type", e.Type).
			Str("old_serial", e.OldSerial).Str("new_serial", e.NewSerial).
			Time("new_not_after", e.NewNotAfter).Msg("Certificate change detected")
	}
}

func newHistoryCmd() *cobra.Command {
	var (
		path   string
		host   string
		typ    string
		since  time.Duration
		events bool
	)
	cmd := &cobra.Command{
		Use:          "history",
		Short:        "Show certificate renewal history",
		SilenceUsage: true,
		Long: `Reads the file written by --history-file and prints a per-target summary,
or the event log with --events. Use -o json for machine-readable output.

Examples:
  cert-monitor history --history-file certs.json
  cert-monitor history --history-file certs.json --events --type renewed --since 720h`,
		Args: cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			p := resolveHistoryPath(path)
			if p == "" {
				return fmt.Errorf("--history-file (or $%s) is required", envHistoryFile)
			}
			store, err := history.Open(p)
			if err != nil {
				return err
			}
			f := history.Filter{Target: host, Type: typ}
			if since > 0 {
				f.Since = time.Now().Add(-since)
			}
			asJSON := viper.GetString("output") == "json"
			if events {
				return printEvents(cmd.OutOrStdout(), store.Events(f), asJSON)
			}
			return printSummary(cmd.OutOrStdout(), store.Summary(host), asJSON)
		},
	}
	cmd.Flags().StringVar(&path, "history-file", "", "History file to read (or $"+envHistoryFile+")")
	cmd.Flags().StringVar(&host, "host", "", "Only this target (as host:port, or namespace/name for k8s)")
	cmd.Flags().StringVar(&typ, "type", "", "Only events of this type: first_seen, renewed, changed (with --events)")
	cmd.Flags().DurationVar(&since, "since", 0, "Only events newer than this (with --events), e.g. 720h")
	cmd.Flags().BoolVar(&events, "events", false, "List events instead of the per-target summary")
	return cmd
}

func printJSON(w io.Writer, v any) error {
	enc := json.NewEncoder(w)
	enc.SetIndent("", "  ")
	return enc.Encode(v)
}

func ts(t time.Time) string {
	if t.IsZero() {
		return "-"
	}
	return t.Format("2006-01-02 15:04")
}

func printEvents(w io.Writer, evs []history.Event, asJSON bool) error {
	if asJSON {
		if evs == nil {
			evs = []history.Event{}
		}
		return printJSON(w, evs)
	}
	tw := tabwriter.NewWriter(w, 0, 0, 2, ' ', 0)
	_, _ = fmt.Fprintln(tw, "TIME\tTARGET\tTYPE\tOLD SERIAL\tNEW SERIAL\tNEW EXPIRY")
	for _, e := range evs {
		_, _ = fmt.Fprintf(tw, "%s\t%s\t%s\t%s\t%s\t%s\n", ts(e.Time), e.Target, e.Type,
			orDash(e.OldSerial), e.NewSerial, ts(e.NewNotAfter))
	}
	return tw.Flush()
}

func printSummary(w io.Writer, rows []history.TargetSummary, asJSON bool) error {
	if asJSON {
		if rows == nil {
			rows = []history.TargetSummary{}
		}
		return printJSON(w, rows)
	}
	tw := tabwriter.NewWriter(w, 0, 0, 2, ' ', 0)
	_, _ = fmt.Fprintln(tw, "TARGET\tEXPIRES\tLAST SEEN\tRENEWALS\tLAST RENEWED")
	for _, r := range rows {
		_, _ = fmt.Fprintf(tw, "%s\t%s\t%s\t%d\t%s\n", r.Target, ts(r.NotAfter), ts(r.LastSeen), r.Renewals, ts(r.LastRenewed))
	}
	return tw.Flush()
}

func orDash(s string) string {
	if s == "" {
		return "-"
	}
	return s
}
