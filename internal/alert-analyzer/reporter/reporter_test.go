package reporter

import (
	"bytes"
	"encoding/json"
	"testing"
	"time"

	"github.com/neogan/sre-toolkit/internal/alert-analyzer/analyzer"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNewReporter(t *testing.T) {
	var buf bytes.Buffer
	r := NewReporter(FormatTable, &buf)
	assert.NotNil(t, r)
	assert.Equal(t, FormatTable, r.format)
	assert.Equal(t, &buf, r.writer)
}

func TestReportSummary(t *testing.T) {
	stats := analyzer.SummaryStats{
		TotalAlerts:        100,
		UniqueAlerts:       50,
		TotalFirings:       150,
		AvgDuration:        5 * time.Minute,
		TotalFiringTime:    12 * time.Hour,
		MostFrequent:       "HighCPU",
		LongestAvgDuration: "BackupFailed",
	}

	t.Run("Table Format", func(t *testing.T) {
		var buf bytes.Buffer
		r := NewReporter(FormatTable, &buf)
		err := r.ReportSummary(stats)
		assert.NoError(t, err)

		output := buf.String()
		assert.Contains(t, output, "=== Alert Analysis Summary ===")
		assert.Contains(t, output, "Total Alert Instances: 100")
		assert.Contains(t, output, "Unique Alerts: 50")
		assert.Contains(t, output, "Most Frequent Alert: HighCPU")
	})

	t.Run("JSON Format", func(t *testing.T) {
		var buf bytes.Buffer
		r := NewReporter(FormatJSON, &buf)
		err := r.ReportSummary(stats)
		assert.NoError(t, err)

		var output map[string]analyzer.SummaryStats
		err = json.Unmarshal(buf.Bytes(), &output)
		require.NoError(t, err)

		assert.Equal(t, stats.TotalAlerts, output["summary"].TotalAlerts)
		assert.Equal(t, stats.MostFrequent, output["summary"].MostFrequent)
	})

	t.Run("Invalid Format", func(t *testing.T) {
		var buf bytes.Buffer
		r := NewReporter("yaml", &buf)
		err := r.ReportSummary(stats)
		assert.Error(t, err)
	})
}

func TestReportFrequency(t *testing.T) {
	results := []analyzer.FrequencyResult{
		{
			AlertName:   "Alert1",
			FiringCount: 10,
			AvgDuration: 2 * time.Minute,
			TotalTime:   20 * time.Minute,
			LastFired:   time.Date(2025, 1, 1, 12, 0, 0, 0, time.UTC),
			Severity:    "critical",
		},
	}

	t.Run("Table Format", func(t *testing.T) {
		var buf bytes.Buffer
		r := NewReporter(FormatTable, &buf)
		err := r.ReportFrequency(results)
		assert.NoError(t, err)

		output := buf.String()
		assert.Contains(t, output, "=== Alert Frequency Analysis ===")
		assert.Contains(t, output, "Alert1")
		assert.Contains(t, output, "10")
		assert.Contains(t, output, "critical")
		assert.Contains(t, output, "🔴")
	})

	t.Run("JSON Format", func(t *testing.T) {
		var buf bytes.Buffer
		r := NewReporter(FormatJSON, &buf)
		err := r.ReportFrequency(results)
		assert.NoError(t, err)

		var output map[string][]analyzer.FrequencyResult
		err = json.Unmarshal(buf.Bytes(), &output)
		require.NoError(t, err)

		assert.Len(t, output["frequency_analysis"], 1)
		assert.Equal(t, "Alert1", output["frequency_analysis"][0].AlertName)
	})

	t.Run("Empty Results", func(t *testing.T) {
		var buf bytes.Buffer
		r := NewReporter(FormatTable, &buf)
		err := r.ReportFrequency([]analyzer.FrequencyResult{})
		assert.NoError(t, err)
		assert.Contains(t, buf.String(), "No alerts found")
	})
}

func TestReportFlapping(t *testing.T) {
	results := []analyzer.FlappingResult{
		{
			AlertName:       "FlappingAlert",
			IsFlapping:      true,
			FlappingScore:   5.5,
			TransitionCount: 10,
			Severity:        "warning",
		},
	}

	t.Run("Table Format", func(t *testing.T) {
		var buf bytes.Buffer
		r := NewReporter(FormatTable, &buf)
		err := r.ReportFlapping(results)
		assert.NoError(t, err)

		output := buf.String()
		assert.Contains(t, output, "=== Flapping Alerts Analysis ===")
		assert.Contains(t, output, "FlappingAlert")
		assert.Contains(t, output, "Yes")
		assert.Contains(t, output, "5.50/hr")
	})
}

func TestReportCompleteWithFlapping(t *testing.T) {
	stats := analyzer.SummaryStats{TotalAlerts: 10}
	freq := []analyzer.FrequencyResult{{AlertName: "A1"}}
	flap := []analyzer.FlappingResult{{AlertName: "A1", IsFlapping: true}}

	t.Run("JSON Format", func(t *testing.T) {
		var buf bytes.Buffer
		r := NewReporter(FormatJSON, &buf)
		err := r.ReportCompleteWithFlapping(stats, freq, flap)
		assert.NoError(t, err)

		var output map[string]interface{}
		err = json.Unmarshal(buf.Bytes(), &output)
		require.NoError(t, err)

		assert.Contains(t, output, "summary")
		assert.Contains(t, output, "frequency_analysis")
		assert.Contains(t, output, "flapping_analysis")
	})
}

func TestFormatDuration(t *testing.T) {
	tests := []struct {
		input    time.Duration
		expected string
	}{
		{0, "0s"},
		{30 * time.Second, "30s"},
		{90 * time.Second, "1m 30s"},
		{60 * time.Minute, "1h 0m"},
		{25 * time.Hour, "1d 1h 0m"},
	}

	for _, tt := range tests {
		assert.Equal(t, tt.expected, formatDuration(tt.input))
	}
}

func TestGetSeverityIcon(t *testing.T) {
	assert.Equal(t, "🔴", getSeverityIcon("critical"))
	assert.Equal(t, "⚠️", getSeverityIcon("warning"))
	assert.Equal(t, "ℹ️", getSeverityIcon("info"))
	assert.Equal(t, "❓", getSeverityIcon("unknown"))
}
