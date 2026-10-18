// Package main provides the entry point for the log-parser tool.
package main

import (
	"bufio"
	"context"
	"fmt"
	"io"
	"os"
	"os/signal"
	"regexp"
	"strings"
	"syscall"
	"time"

	"github.com/neogan/sre-toolkit/internal/log-parser/analyzer"
	"github.com/neogan/sre-toolkit/internal/log-parser/follow"
	"github.com/neogan/sre-toolkit/internal/log-parser/formats"
	"github.com/neogan/sre-toolkit/internal/log-parser/reporter"
	"github.com/neogan/sre-toolkit/pkg/logging"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"
)

func main() {
	logging.Init(nil)

	rootCmd := &cobra.Command{
		Use:   "log-parser",
		Short: "Intelligent log analyzer with pattern matching and anomaly detection",
		Long: `log-parser parses and analyzes log files in multiple formats (JSON, logfmt,
Apache/nginx access logs, syslog, plaintext). It provides statistics, detects
error spikes, matches user-defined patterns, and exports results as JSON.`,
	}

	rootCmd.PersistentFlags().String("output", "table", "Output format (table, json)")
	_ = viper.BindPFlag("output", rootCmd.PersistentFlags().Lookup("output"))

	rootCmd.AddCommand(
		newAnalyzeCmd(),
		newGrepCmd(),
		newTailCmd(),
		newTraceCmd(),
		newVersionCmd(),
	)

	if err := rootCmd.Execute(); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}

// newAnalyzeCmd creates the `analyze` subcommand.
func newAnalyzeCmd() *cobra.Command {
	var (
		format          string
		patterns        []string
		topN            int
		anomalyWindow   time.Duration
		anomalyMinCount int
		traceKeys       []string
	)

	cmd := &cobra.Command{
		Use:   "analyze [file...]",
		Short: "Analyze log files and show statistics",
		Long: `Parse one or more log files and output aggregated statistics:
level breakdown, top messages, top errors, pattern matches, and anomalies.
Reads from stdin if no files are provided.

Examples:
  log-parser analyze app.log
  log-parser analyze --output json app.log
  log-parser analyze --pattern "timeout" --pattern "connection refused" app.log
  cat app.log | log-parser analyze`,
		RunE: func(cmd *cobra.Command, args []string) error {
			outputFmt := reporter.Format(viper.GetString("output"))
			rep := reporter.New(outputFmt, os.Stdout)

			cfg := &analyzer.Config{
				Format:          format,
				Patterns:        patterns,
				TopN:            topN,
				AnomalyWindow:   anomalyWindow,
				AnomalyMinCount: anomalyMinCount,
				TraceKeys:       traceKeys,
			}

			r, closer, err := openInput(args)
			if err != nil {
				return err
			}
			defer closer()

			stats, err := analyzer.Analyze(cmd.Context(), r, cfg)
			if err != nil {
				return fmt.Errorf("analyzing logs: %w", err)
			}

			return rep.Report(stats)
		},
	}

	cmd.Flags().StringVar(&format, "format", "", "Force log format: json, logfmt, access, syslog, plain (auto-detect if empty)")
	cmd.Flags().StringArrayVar(&patterns, "pattern", nil, "Regex pattern to track (repeatable)")
	cmd.Flags().IntVar(&topN, "top", 10, "Number of top messages to show")
	cmd.Flags().DurationVar(&anomalyWindow, "anomaly-window", time.Minute, "Time window for anomaly detection")
	cmd.Flags().IntVar(&anomalyMinCount, "anomaly-min", 5, "Minimum errors in a window to flag as anomaly")
	cmd.Flags().StringArrayVar(&traceKeys, "trace-key", nil, "Field holding the trace/request ID (repeatable, default: trace_id, request_id, correlation_id, req_id, x_request_id)")

	return cmd
}

// newGrepCmd creates the `grep` subcommand — filtered log output.
func newGrepCmd() *cobra.Command {
	var (
		level   string
		format  string
		lineNum bool
	)

	cmd := &cobra.Command{
		Use:   "grep <pattern> [file...]",
		Short: "Filter log lines matching a regex pattern or level",
		Long: `Filter and print log lines matching a regex pattern, optionally
filtering by minimum severity level. Reads from stdin if no files provided.

Examples:
  log-parser grep "database" app.log
  log-parser grep --level error app.log
  log-parser grep "timeout" --level warn app.log`,
		Args: cobra.MinimumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			pattern := args[0]
			files := args[1:]

			re, err := regexp.Compile(pattern)
			if err != nil {
				return fmt.Errorf("invalid pattern %q: %w", pattern, err)
			}

			minLevel := formats.LevelUnknown
			if level != "" {
				minLevel = parseLevelFlag(level)
			}

			r, closer, err := openInput(files)
			if err != nil {
				return err
			}
			defer closer()

			var parser formats.Parser
			sc := bufio.NewScanner(r)
			sc.Buffer(make([]byte, 1*1024*1024), 1*1024*1024)
			total, matched := 0, 0

			for sc.Scan() {
				total++
				line := sc.Text()
				if strings.TrimSpace(line) == "" {
					continue
				}
				if parser == nil {
					parser = detectParser(format, line)
				}

				entry, err := parser.Parse(line)
				if err != nil {
					continue
				}
				entry.LineNum = total

				if minLevel != formats.LevelUnknown && !levelAtLeast(entry.Level, minLevel) {
					continue
				}
				if !re.MatchString(line) {
					continue
				}

				matched++
				ts := ""
				if !entry.Timestamp.IsZero() {
					ts = entry.Timestamp.Format("2006-01-02 15:04:05") + " "
				}
				lineStr := ""
				if lineNum {
					lineStr = fmt.Sprintf("L%-5d ", entry.LineNum)
				}
				fmt.Fprintf(os.Stdout, "%s%s[%s] %s\n", lineStr, ts, levelColor(entry.Level), entry.Message) //nolint:gosec // writing to stdout, not an HTTP context
			}

			fmt.Fprintf(os.Stderr, "\n%d/%d lines matched\n", matched, total)
			return sc.Err()
		},
	}

	cmd.Flags().StringVar(&level, "level", "", "Minimum log level (debug, info, warn, error, fatal)")
	cmd.Flags().StringVar(&format, "format", "", "Force log format")
	cmd.Flags().BoolVarP(&lineNum, "line-number", "n", false, "Show line numbers")

	return cmd
}

// newTailCmd creates the `tail` subcommand — show last N parsed entries.
func newTailCmd() *cobra.Command {
	var (
		n          int
		level      string
		format     string
		followMode bool
	)

	cmd := &cobra.Command{
		Use:   "tail [file...]",
		Short: "Show last N parsed log entries",
		Long: `Display the last N log entries from a file, with parsed level and timestamp.
With -f, keep running and print entries as they are appended; truncation and
log rotation are handled. Stop with Ctrl-C.

Examples:
  log-parser tail -n 20 app.log
  log-parser tail --level error app.log
  log-parser tail -f --level warn app.log`,
		RunE: func(cmd *cobra.Command, args []string) error {
			if followMode && len(args) != 1 {
				return fmt.Errorf("--follow requires exactly one file")
			}
			minLevel := formats.LevelUnknown
			if level != "" {
				minLevel = parseLevelFlag(level)
			}

			r, closer, err := openInput(args)
			if err != nil {
				return err
			}
			defer closer()
			cr := &countingReader{r: r}

			tl := &tailer{format: format, minLevel: minLevel}
			var entries []*formats.Entry
			sc := bufio.NewScanner(cr)
			sc.Buffer(make([]byte, 1*1024*1024), 1*1024*1024)

			for sc.Scan() {
				if entry := tl.parse(sc.Text()); entry != nil {
					entries = append(entries, entry)
					if len(entries) > n {
						entries = entries[1:]
					}
				}
			}
			if err := sc.Err(); err != nil {
				return err
			}

			for _, e := range entries {
				printEntry(e)
			}

			if !followMode {
				return nil
			}
			ctx, stop := signal.NotifyContext(cmd.Context(), os.Interrupt, syscall.SIGTERM)
			defer stop()
			return follow.Lines(ctx, args[0], cr.n, follow.DefaultInterval, func(line string) {
				if entry := tl.parse(line); entry != nil {
					printEntry(entry)
				}
			})
		},
	}

	cmd.Flags().IntVarP(&n, "lines", "n", 20, "Number of lines to show")
	cmd.Flags().StringVar(&level, "level", "", "Minimum log level")
	cmd.Flags().StringVar(&format, "format", "", "Force log format")
	cmd.Flags().BoolVarP(&followMode, "follow", "f", false, "Keep printing entries appended to the file")

	return cmd
}

// tailer parses lines one at a time, keeping the detected parser and line number.
type tailer struct {
	format   string
	minLevel formats.Level
	parser   formats.Parser
	lineN    int
}

// parse returns nil for blank, unparsable or filtered-out lines.
func (t *tailer) parse(line string) *formats.Entry {
	t.lineN++
	if strings.TrimSpace(line) == "" {
		return nil
	}
	if t.parser == nil {
		t.parser = detectParser(t.format, line)
	}
	entry, err := t.parser.Parse(line)
	if err != nil {
		return nil
	}
	entry.LineNum = t.lineN
	if t.minLevel != formats.LevelUnknown && !levelAtLeast(entry.Level, t.minLevel) {
		return nil
	}
	return entry
}

// countingReader counts bytes read, giving the offset to continue following from.
type countingReader struct {
	r io.Reader
	n int64
}

func (c *countingReader) Read(p []byte) (int, error) {
	n, err := c.r.Read(p)
	c.n += int64(n)
	return n, err
}

func printEntry(e *formats.Entry) {
	ts := ""
	if !e.Timestamp.IsZero() {
		ts = e.Timestamp.Format("2006-01-02 15:04:05") + " "
	}
	fmt.Fprintf(os.Stdout, "L%-5d %s[%s] %s\n", e.LineNum, ts, levelColor(e.Level), e.Message) //nolint:gosec // writing to stdout, not an HTTP context
}

// newTraceCmd creates the `trace` subcommand — every line of one request.
func newTraceCmd() *cobra.Command {
	var (
		format    string
		traceKeys []string
	)

	cmd := &cobra.Command{
		Use:   "trace <id> [file...]",
		Short: "Show every log line belonging to one trace/request ID",
		Long: `Print all entries of a single trace in file order. A line belongs to the
trace when its trace/request ID field equals <id>; in plain-text logs without
such a field, any line containing <id> is included.

Multiple files are read in the order given, so pass service logs together to
follow a request across them. Reads stdin if no files are provided.

Examples:
  log-parser trace 4bf92f3577b34da6 api.log worker.log
  log-parser trace --trace-key order_id 1042 app.log`,
		Args: cobra.MinimumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			id, files := args[0], args[1:]
			r, closer, err := openInput(files)
			if err != nil {
				return err
			}
			defer closer()

			tl := &tailer{format: format, minLevel: formats.LevelUnknown}
			sc := bufio.NewScanner(r)
			sc.Buffer(make([]byte, 1*1024*1024), 1*1024*1024)
			matched := 0
			for sc.Scan() {
				entry := tl.parse(sc.Text())
				if entry == nil || !belongsToTrace(entry, id, traceKeys) {
					continue
				}
				matched++
				printEntry(entry)
			}
			if err := sc.Err(); err != nil {
				return err
			}
			fmt.Fprintf(os.Stderr, "\n%d lines in trace %s\n", matched, id)
			return nil
		},
	}

	cmd.Flags().StringVar(&format, "format", "", "Force log format")
	cmd.Flags().StringArrayVar(&traceKeys, "trace-key", nil, "Field holding the trace/request ID (repeatable)")

	return cmd
}

// belongsToTrace reports whether e is part of trace id: its ID field matches,
// or, when it has no ID field at all, the raw line mentions id.
func belongsToTrace(e *formats.Entry, id string, keys []string) bool {
	if got := e.TraceID(keys); got != "" {
		return got == id
	}
	return strings.Contains(e.Raw, id)
}

func newVersionCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "version",
		Short: "Print version information",
		Run: func(cmd *cobra.Command, args []string) {
			fmt.Println("log-parser version 0.6.0")
		},
	}
}

// openInput returns a reader over the given files (or stdin), plus a closer.
func openInput(files []string) (io.Reader, func(), error) {
	if len(files) == 0 {
		return os.Stdin, func() {}, nil
	}
	if len(files) == 1 {
		f, err := os.Open(files[0]) //nolint:gosec // file path is provided by user via CLI argument
		if err != nil {
			return nil, func() {}, err
		}
		return f, func() {
			if err := f.Close(); err != nil {
				logger := logging.GetLogger()
				logger.Warn().Err(err).Msg("Failed to close file")
			}
		}, nil
	}
	readers := make([]io.Reader, 0, len(files))
	closers := make([]io.Closer, 0, len(files))
	for _, name := range files {
		f, err := os.Open(name) //nolint:gosec // file path is provided by user via CLI argument
		if err != nil {
			for _, c := range closers {
				c.Close()
			}
			return nil, func() {}, err
		}
		readers = append(readers, f)
		closers = append(closers, f)
	}
	return io.MultiReader(readers...), func() {
		for _, c := range closers {
			c.Close()
		}
	}, nil
}

func detectParser(format, line string) formats.Parser {
	if format != "" {
		for _, p := range formats.All() {
			if p.Name() == format {
				return p
			}
		}
	}
	return formats.Detect(line)
}

func parseLevelFlag(s string) formats.Level {
	switch strings.ToLower(s) {
	case "trace":
		return formats.LevelTrace
	case "debug":
		return formats.LevelDebug
	case "info":
		return formats.LevelInfo
	case "warn", "warning":
		return formats.LevelWarning
	case "error", "err":
		return formats.LevelError
	case "fatal", "critical":
		return formats.LevelFatal
	default:
		return formats.LevelUnknown
	}
}

var levelOrder = map[formats.Level]int{
	formats.LevelTrace:   0,
	formats.LevelDebug:   1,
	formats.LevelInfo:    2,
	formats.LevelWarning: 3,
	formats.LevelError:   4,
	formats.LevelFatal:   5,
	formats.LevelUnknown: -1,
}

func levelAtLeast(got, minLevel formats.Level) bool {
	return levelOrder[got] >= levelOrder[minLevel]
}

func levelColor(lvl formats.Level) string {
	switch lvl {
	case formats.LevelFatal:
		return "\033[35mFATAL\033[0m"
	case formats.LevelError:
		return "\033[31mERROR\033[0m"
	case formats.LevelWarning:
		return "\033[33mWARN\033[0m"
	case formats.LevelInfo:
		return "\033[32mINFO\033[0m"
	case formats.LevelDebug:
		return "\033[36mDEBUG\033[0m"
	default:
		return "\033[90m" + string(lvl) + "\033[0m"
	}
}

// Ensure context import is used (used in analyzer.Analyze signature).
var _ = context.Background
