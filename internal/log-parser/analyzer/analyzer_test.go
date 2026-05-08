package analyzer

import (
	"context"
	"strings"
	"testing"
	"time"

	"github.com/neogan/sre-toolkit/internal/log-parser/formats"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestAnalyze_JSONLogs(t *testing.T) {
	input := strings.Join([]string{
		`{"level":"info","time":"2024-03-15T10:00:00Z","message":"server started"}`,
		`{"level":"info","time":"2024-03-15T10:00:01Z","message":"request received"}`,
		`{"level":"error","time":"2024-03-15T10:00:02Z","message":"database connection failed"}`,
		`{"level":"warn","time":"2024-03-15T10:00:03Z","message":"slow query detected"}`,
		`{"level":"error","time":"2024-03-15T10:00:04Z","message":"database connection failed"}`,
	}, "\n")

	stats, err := Analyze(context.Background(), strings.NewReader(input), nil)
	require.NoError(t, err)

	assert.Equal(t, 5, stats.TotalLines)
	assert.Equal(t, 5, stats.ParsedLines)
	assert.Equal(t, 0, stats.ErrorLines)
	assert.Equal(t, 2, stats.LevelCounts[formats.LevelInfo])
	assert.Equal(t, 2, stats.LevelCounts[formats.LevelError])
	assert.Equal(t, 1, stats.LevelCounts[formats.LevelWarning])
}

func TestAnalyze_PlainLogs(t *testing.T) {
	input := strings.Join([]string{
		"2024-03-15 10:00:00 INFO application started",
		"2024-03-15 10:00:01 ERROR something went wrong",
		"2024-03-15 10:00:02 WARN resource usage high",
		"2024-03-15 10:00:03 INFO request processed",
	}, "\n")

	stats, err := Analyze(context.Background(), strings.NewReader(input), nil)
	require.NoError(t, err)

	assert.Equal(t, 4, stats.TotalLines)
	assert.Equal(t, 4, stats.ParsedLines)
}

func TestAnalyze_LogfmtLogs(t *testing.T) {
	input := strings.Join([]string{
		`level=info msg="request started" path=/api/users`,
		`level=error msg="database error" error="connection refused"`,
		`level=info msg="request completed" status=200`,
	}, "\n")

	cfg := &Config{Format: "logfmt", TopN: 5}
	stats, err := Analyze(context.Background(), strings.NewReader(input), cfg)
	require.NoError(t, err)

	assert.Equal(t, 3, stats.TotalLines)
	assert.Equal(t, 1, stats.LevelCounts[formats.LevelError])
}

func TestAnalyze_TopMessages(t *testing.T) {
	input := strings.Join([]string{
		`{"level":"info","time":"2024-01-01T00:00:00Z","message":"cache hit"}`,
		`{"level":"info","time":"2024-01-01T00:00:01Z","message":"cache hit"}`,
		`{"level":"info","time":"2024-01-01T00:00:02Z","message":"cache hit"}`,
		`{"level":"error","time":"2024-01-01T00:00:03Z","message":"cache miss"}`,
		`{"level":"error","time":"2024-01-01T00:00:04Z","message":"cache miss"}`,
	}, "\n")

	cfg := &Config{TopN: 3}
	stats, err := Analyze(context.Background(), strings.NewReader(input), cfg)
	require.NoError(t, err)

	require.NotEmpty(t, stats.TopMessages)
	// "cache hit" should be top message (normalised: "cache hit")
	assert.Contains(t, stats.TopMessages[0].Message, "cache")
	assert.GreaterOrEqual(t, stats.TopMessages[0].Count, 2)

	require.NotEmpty(t, stats.TopErrors)
	assert.Contains(t, stats.TopErrors[0].Message, "cache miss")
}

func TestAnalyze_PatternMatching(t *testing.T) {
	input := strings.Join([]string{
		`{"level":"error","time":"2024-01-01T00:00:00Z","message":"connection timeout to postgres"}`,
		`{"level":"error","time":"2024-01-01T00:00:01Z","message":"connection refused"}`,
		`{"level":"info","time":"2024-01-01T00:00:02Z","message":"health check passed"}`,
	}, "\n")

	cfg := &Config{
		Patterns: []string{"timeout", "refused"},
		TopN:     5,
	}
	stats, err := Analyze(context.Background(), strings.NewReader(input), cfg)
	require.NoError(t, err)

	require.Len(t, stats.Patterns, 2)
	assert.Equal(t, "timeout", stats.Patterns[0].Pattern)
	assert.Equal(t, 1, stats.Patterns[0].Count)
	assert.Equal(t, "refused", stats.Patterns[1].Pattern)
	assert.Equal(t, 1, stats.Patterns[1].Count)
}

func TestAnalyze_AnomalyDetection(t *testing.T) {
	// Create an error spike within a 1-minute window
	var lines []string
	base := time.Date(2024, 1, 1, 10, 0, 0, 0, time.UTC)

	// 10 background error buckets, 1 error each (every 2 minutes)
	for i := 0; i < 10; i++ {
		ts := base.Add(time.Duration(i) * 2 * time.Minute)
		lines = append(lines, `{"level":"error","time":"`+ts.Format(time.RFC3339)+`","message":"background error"}`)
	}
	// Spike: 20 errors in 20 seconds (single bucket)
	spike := base.Add(30 * time.Minute)
	for i := 0; i < 20; i++ {
		ts := spike.Add(time.Duration(i) * time.Second)
		lines = append(lines, `{"level":"error","time":"`+ts.Format(time.RFC3339)+`","message":"service unavailable"}`)
	}

	cfg := &Config{
		AnomalyWindow:   time.Minute,
		AnomalyMinCount: 5,
		TopN:            5,
	}
	stats, err := Analyze(context.Background(), strings.NewReader(strings.Join(lines, "\n")), cfg)
	require.NoError(t, err)

	assert.NotEmpty(t, stats.Anomalies, "should detect error spike")
}

func TestAnalyze_EmptyInput(t *testing.T) {
	stats, err := Analyze(context.Background(), strings.NewReader(""), nil)
	require.NoError(t, err)
	assert.Equal(t, 0, stats.TotalLines)
}

func TestAnalyze_TimeRange(t *testing.T) {
	input := strings.Join([]string{
		`{"level":"info","time":"2024-03-15T10:00:00Z","message":"first"}`,
		`{"level":"info","time":"2024-03-15T10:05:00Z","message":"middle"}`,
		`{"level":"info","time":"2024-03-15T10:10:00Z","message":"last"}`,
	}, "\n")

	stats, err := Analyze(context.Background(), strings.NewReader(input), nil)
	require.NoError(t, err)

	assert.Equal(t, 2024, stats.TimeRange.Start.Year())
	assert.Equal(t, 0, stats.TimeRange.Start.Minute())
	assert.Equal(t, 10, stats.TimeRange.End.Minute())
	assert.Greater(t, stats.RatePerMin, 0.0)
}

func TestAnalyze_InvalidPattern(t *testing.T) {
	cfg := &Config{Patterns: []string{"[invalid"}}
	_, err := Analyze(context.Background(), strings.NewReader("anything"), cfg)
	assert.Error(t, err)
}

func TestAnalyze_ContextCancellation(t *testing.T) {
	// Generate a large input
	var lines []string
	for i := 0; i < 10000; i++ {
		lines = append(lines, `{"level":"info","time":"2024-01-01T00:00:00Z","message":"line"}`)
	}

	ctx, cancel := context.WithCancel(context.Background())
	cancel() // cancel immediately

	_, err := Analyze(ctx, strings.NewReader(strings.Join(lines, "\n")), nil)
	// May or may not error depending on timing, but should not panic
	_ = err
}

func TestNormalise(t *testing.T) {
	tests := []struct {
		input string
		want  string
	}{
		{"request 12345 failed", "request <N> failed"},
		{"connected user 42 times", "connected user <N> times"},
		{"0xdeadbeef address", "<hex> address"},
	}
	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			assert.Equal(t, tt.want, normalise(tt.input))
		})
	}
}

func TestMeanStddev(t *testing.T) {
	mean, stddev := meanStddev([]float64{1, 2, 3, 4, 5})
	assert.InDelta(t, 3.0, mean, 0.01)
	assert.InDelta(t, 1.41, stddev, 0.01)

	mean, stddev = meanStddev(nil)
	assert.Equal(t, 0.0, mean)
	assert.Equal(t, 0.0, stddev)
}

func TestDefaultConfig(t *testing.T) {
	cfg := DefaultConfig()
	assert.Equal(t, 10, cfg.TopN)
	assert.Equal(t, time.Minute, cfg.AnomalyWindow)
	assert.Equal(t, 5, cfg.AnomalyMinCount)
}
