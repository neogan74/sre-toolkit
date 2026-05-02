// Package main provides the entry point for the log-parser tool.
package main

import (
	"bufio"
	"context"
	"fmt"
	"io"
	"os"
	"regexp"
	"strings"
	"time"

	"github.com/neogan/sre-toolkit/internal/log-parser/analyzer"
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
				fmt.Fprintf(os.Stdout, "%s%s[%s] %s\n", lineStr, ts, levelColor(entry.Level), entry.Message)
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
		n      int
		level  string
		format string
	)

	cmd := &cobra.Command{
		Use:   "tail [file...]",
		Short: "Show last N parsed log entries",
		Long: `Display the last N log entries from a file, with parsed level and timestamp.

Examples:
  log-parser tail -n 20 app.log
  log-parser tail --level error app.log`,
		RunE: func(cmd *cobra.Command, args []string) error {
			minLevel := formats.LevelUnknown
			if level != "" {
				minLevel = parseLevelFlag(level)
			}

			r, closer, err := openInput(args)
			if err != nil {
				return err
			}
			defer closer()

			var parser formats.Parser
			var entries []*formats.Entry
			sc := bufio.NewScanner(r)
			sc.Buffer(make([]byte, 1*1024*1024), 1*1024*1024)
			lineN := 0

			for sc.Scan() {
				lineN++
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
				entry.LineNum = lineN

				if minLevel != formats.LevelUnknown && !levelAtLeast(entry.Level, minLevel) {
					continue
				}

				entries = append(entries, entry)
				if len(entries) > n {
					entries = entries[1:]
				}
			}

			for _, e := range entries {
				ts := ""
				if !e.Timestamp.IsZero() {
					ts = e.Timestamp.Format("2006-01-02 15:04:05") + " "
				}
				fmt.Fprintf(os.Stdout, "L%-5d %s[%s] %s\n", e.LineNum, ts, levelColor(e.Level), e.Message)
			}

			return sc.Err()
		},
	}

	cmd.Flags().IntVarP(&n, "lines", "n", 20, "Number of lines to show")
	cmd.Flags().StringVar(&level, "level", "", "Minimum log level")
	cmd.Flags().StringVar(&format, "format", "", "Force log format")

	return cmd
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
		f, err := os.Open(files[0])
		if err != nil {
			return nil, func() {}, err
		}
		return f, func() {
			if err := f.Close(); err != nil {
				logging.GetLogger().Warn().Err(err).Msg("Failed to close file")
			}
		}, nil
	}
	readers := make([]io.Reader, 0, len(files))
	closers := make([]io.Closer, 0, len(files))
	for _, name := range files {
		f, err := os.Open(name)
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

func levelAtLeast(got, min formats.Level) bool {
	return levelOrder[got] >= levelOrder[min]
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
