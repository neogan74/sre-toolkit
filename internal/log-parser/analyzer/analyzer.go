// Package analyzer provides log analysis: pattern matching, anomaly detection, stats.
package analyzer

import (
	"bufio"
	"context"
	"io"
	"math"
	"regexp"
	"sort"
	"strings"
	"time"

	"github.com/neogan/sre-toolkit/internal/log-parser/formats"
)

// Stats holds aggregated statistics from a log stream.
type Stats struct {
	TotalLines  int
	ParsedLines int
	ErrorLines  int
	LevelCounts map[formats.Level]int
	TopMessages []MessageCount
	TopErrors   []MessageCount
	Patterns    []PatternMatch
	Anomalies   []Anomaly
	TimeRange   TimeRange
	RatePerMin  float64
}

// MessageCount pairs a message template with its occurrence count.
type MessageCount struct {
	Message string
	Count   int
}

// PatternMatch represents a user-defined pattern hit.
type PatternMatch struct {
	Pattern string
	Count   int
	Entries []*formats.Entry
}

// Anomaly represents a detected spike in error rate.
type Anomaly struct {
	Time     time.Time
	Message  string
	Count    int
	Severity formats.Level
}

// TimeRange holds the first and last timestamps seen.
type TimeRange struct {
	Start time.Time
	End   time.Time
}

// Config controls analyzer behaviour.
type Config struct {
	Patterns        []string // regex patterns to track
	TopN            int      // how many top messages to show
	AnomalyWindow   time.Duration
	AnomalyMinCount int    // min errors in window to flag anomaly
	Format          string // "", "json", "logfmt", "access", "syslog", "plain"
}

// DefaultConfig returns sensible defaults.
func DefaultConfig() *Config {
	return &Config{
		TopN:            10,
		AnomalyWindow:   time.Minute,
		AnomalyMinCount: 5,
	}
}

// Analyze reads lines from r, parses them, and returns aggregated stats.
func Analyze(ctx context.Context, r io.Reader, cfg *Config) (*Stats, error) {
	if cfg == nil {
		cfg = DefaultConfig()
	}

	// Compile patterns
	var compiledPatterns []*regexp.Regexp
	patternCounts := make([]int, len(cfg.Patterns))
	patternEntries := make([][]*formats.Entry, len(cfg.Patterns))
	for i, p := range cfg.Patterns {
		re, err := regexp.Compile(p)
		if err != nil {
			return nil, err
		}
		compiledPatterns = append(compiledPatterns, re)
		patternEntries[i] = nil
	}

	stats := &Stats{
		LevelCounts: make(map[formats.Level]int),
	}

	msgCounts := make(map[string]int)
	errCounts := make(map[string]int)

	var buckets []errorBucket
	currentBucket := errorBucket{}

	var parser formats.Parser
	var lineNum int

	scanner := bufio.NewScanner(r)
	// Handle large lines (e.g. long JSON)
	scanner.Buffer(make([]byte, 1*1024*1024), 1*1024*1024)

	for scanner.Scan() {
		select {
		case <-ctx.Done():
			return stats, ctx.Err()
		default:
		}

		lineNum++
		line := scanner.Text()
		if strings.TrimSpace(line) == "" {
			continue
		}
		stats.TotalLines++

		// Auto-detect or use configured parser on first line
		if parser == nil {
			if cfg.Format != "" {
				parser = parserByName(cfg.Format)
			} else {
				parser = formats.Detect(line)
			}
		}

		entry, err := parser.Parse(line)
		if err != nil {
			stats.ErrorLines++
			continue
		}
		entry.LineNum = lineNum
		stats.ParsedLines++
		stats.LevelCounts[entry.Level]++

		// Track time range
		if !entry.Timestamp.IsZero() {
			if stats.TimeRange.Start.IsZero() || entry.Timestamp.Before(stats.TimeRange.Start) {
				stats.TimeRange.Start = entry.Timestamp
			}
			if entry.Timestamp.After(stats.TimeRange.End) {
				stats.TimeRange.End = entry.Timestamp
			}
		}

		// Count messages (normalised)
		key := normalise(entry.Message)
		msgCounts[key]++
		if entry.Level == formats.LevelError || entry.Level == formats.LevelFatal {
			errCounts[key]++
		}

		// Pattern matching
		for i, re := range compiledPatterns {
			if re.MatchString(line) {
				patternCounts[i]++
				if len(patternEntries[i]) < 20 {
					patternEntries[i] = append(patternEntries[i], entry)
				}
			}
		}

		// Anomaly bucketing
		if !entry.Timestamp.IsZero() && (entry.Level == formats.LevelError || entry.Level == formats.LevelFatal) {
			if currentBucket.start.IsZero() {
				currentBucket.start = entry.Timestamp
			}
			if entry.Timestamp.Sub(currentBucket.start) > cfg.AnomalyWindow {
				buckets = append(buckets, currentBucket)
				currentBucket = errorBucket{start: entry.Timestamp}
			}
			currentBucket.errors++
			currentBucket.msgs = append(currentBucket.msgs, entry.Message)
		}
	}

	if err := scanner.Err(); err != nil {
		return stats, err
	}

	// Flush last bucket
	if currentBucket.errors > 0 {
		buckets = append(buckets, currentBucket)
	}

	// Compute rate
	if !stats.TimeRange.Start.IsZero() && !stats.TimeRange.End.IsZero() {
		dur := stats.TimeRange.End.Sub(stats.TimeRange.Start).Minutes()
		if dur > 0 {
			stats.RatePerMin = float64(stats.ParsedLines) / dur
		}
	}

	// Top messages
	stats.TopMessages = topN(msgCounts, cfg.TopN)
	stats.TopErrors = topN(errCounts, cfg.TopN)

	// Pattern results
	for i, p := range cfg.Patterns {
		if patternCounts[i] > 0 {
			stats.Patterns = append(stats.Patterns, PatternMatch{
				Pattern: p,
				Count:   patternCounts[i],
				Entries: patternEntries[i],
			})
		}
	}

	// Anomaly detection: find buckets with significantly more errors than average
	stats.Anomalies = detectAnomalies(buckets, cfg.AnomalyMinCount)

	return stats, nil
}

// errorBucket groups errors within a time window.
type errorBucket struct {
	start  time.Time
	errors int
	msgs   []string
}

// detectAnomalies finds time windows with error counts exceeding mean + 2*stddev.
func detectAnomalies(buckets []errorBucket, minCount int) []Anomaly {
	if len(buckets) == 0 {
		return nil
	}

	counts := make([]float64, len(buckets))
	for i, b := range buckets {
		counts[i] = float64(b.errors)
	}

	mean, stddev := meanStddev(counts)
	threshold := math.Max(float64(minCount), mean+2*stddev)

	var anomalies []Anomaly
	for _, b := range buckets {
		if float64(b.errors) >= threshold && b.errors >= minCount {
			msg := "error spike detected"
			if len(b.msgs) > 0 {
				msg = b.msgs[0]
			}
			anomalies = append(anomalies, Anomaly{
				Time:     b.start,
				Message:  msg,
				Count:    b.errors,
				Severity: formats.LevelError,
			})
		}
	}
	return anomalies
}

func meanStddev(values []float64) (mean, stddev float64) {
	if len(values) == 0 {
		return 0, 0
	}
	for _, v := range values {
		mean += v
	}
	mean /= float64(len(values))
	for _, v := range values {
		d := v - mean
		stddev += d * d
	}
	stddev = math.Sqrt(stddev / float64(len(values)))
	return
}

func topN(counts map[string]int, n int) []MessageCount {
	type kv struct {
		k string
		v int
	}
	sorted := make([]kv, 0, len(counts))
	for k, v := range counts {
		sorted = append(sorted, kv{k, v})
	}
	sort.Slice(sorted, func(i, j int) bool { return sorted[i].v > sorted[j].v })
	if n > len(sorted) {
		n = len(sorted)
	}
	result := make([]MessageCount, n)
	for i := 0; i < n; i++ {
		result[i] = MessageCount{Message: sorted[i].k, Count: sorted[i].v}
	}
	return result
}

// normalise reduces a log message to a template by replacing numbers and UUIDs.
var (
	digitRE = regexp.MustCompile(`\b\d+\b`)
	uuidRE  = regexp.MustCompile(`[0-9a-f]{8}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{12}`)
	hexRE   = regexp.MustCompile(`\b0x[0-9a-f]+\b`)
)

func normalise(msg string) string {
	msg = uuidRE.ReplaceAllString(msg, "<uuid>")
	msg = hexRE.ReplaceAllString(msg, "<hex>")
	msg = digitRE.ReplaceAllString(msg, "<N>")
	// Truncate very long messages
	if len(msg) > 120 {
		msg = msg[:120] + "..."
	}
	return msg
}

func parserByName(name string) formats.Parser {
	for _, p := range formats.All() {
		if p.Name() == name {
			return p
		}
	}
	return &formats.PlainParser{}
}
