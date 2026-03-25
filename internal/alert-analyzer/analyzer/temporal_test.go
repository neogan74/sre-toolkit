package analyzer

import (
	"testing"
	"time"

	"github.com/neogan/sre-toolkit/internal/alert-analyzer/collector"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestTemporalAnalyzer_Analyze(t *testing.T) {
	alerts := []collector.Alert{
		{
			Name:    "BusinessHoursAlert",
			Labels:  map[string]string{"severity": "warning"},
			FiredAt: time.Date(2026, 3, 16, 10, 0, 0, 0, time.UTC), // Monday
		},
		{
			Name:    "BusinessHoursAlert",
			Labels:  map[string]string{"severity": "warning"},
			FiredAt: time.Date(2026, 3, 16, 10, 30, 0, 0, time.UTC), // Monday
		},
		{
			Name:    "BusinessHoursAlert",
			Labels:  map[string]string{"severity": "warning"},
			FiredAt: time.Date(2026, 3, 18, 11, 0, 0, 0, time.UTC), // Wednesday
		},
		{
			Name:    "WeekendAlert",
			Labels:  map[string]string{"severity": "critical"},
			FiredAt: time.Date(2026, 3, 21, 23, 0, 0, 0, time.UTC), // Saturday
		},
	}

	history := &collector.AlertHistory{Alerts: alerts}
	analyzer := NewTemporalAnalyzer(history)
	results := analyzer.Analyze()

	require.Len(t, results, 2)

	assert.Equal(t, "BusinessHoursAlert", results[0].AlertName)
	assert.Equal(t, 3, results[0].TotalFirings)
	assert.Equal(t, 10, results[0].PeakHour)
	assert.Equal(t, 2, results[0].PeakHourCount)
	assert.Equal(t, "Monday", results[0].PeakWeekday)
	assert.Equal(t, 2, results[0].PeakWeekdayCount)
	assert.InDelta(t, 1.0, results[0].BusinessHoursRatio, 0.001)
	assert.InDelta(t, 0.0, results[0].WeekendRatio, 0.001)
	assert.Equal(t, 2, results[0].HourlyDistribution[10])
	assert.Equal(t, 1, results[0].HourlyDistribution[11])

	assert.Equal(t, "WeekendAlert", results[1].AlertName)
	assert.Equal(t, "Saturday", results[1].PeakWeekday)
	assert.InDelta(t, 1.0, results[1].WeekendRatio, 0.001)
	assert.InDelta(t, 0.0, results[1].BusinessHoursRatio, 0.001)
}

func TestTemporalAnalyzer_AnalyzeTopN(t *testing.T) {
	history := &collector.AlertHistory{
		Alerts: []collector.Alert{
			{Name: "A", FiredAt: time.Date(2026, 3, 16, 10, 0, 0, 0, time.UTC)},
			{Name: "A", FiredAt: time.Date(2026, 3, 16, 11, 0, 0, 0, time.UTC)},
			{Name: "B", FiredAt: time.Date(2026, 3, 16, 12, 0, 0, 0, time.UTC)},
		},
	}

	results := NewTemporalAnalyzer(history).AnalyzeTopN(1)
	require.Len(t, results, 1)
	assert.Equal(t, "A", results[0].AlertName)
}

func TestTemporalAnalyzer_AnalyzeEmpty(t *testing.T) {
	assert.Empty(t, NewTemporalAnalyzer(&collector.AlertHistory{}).Analyze())
	assert.Empty(t, NewTemporalAnalyzer(nil).Analyze())
}

func TestIsBusinessHour(t *testing.T) {
	assert.True(t, isBusinessHour(time.Date(2026, 3, 16, 9, 0, 0, 0, time.UTC)))
	assert.True(t, isBusinessHour(time.Date(2026, 3, 16, 17, 59, 0, 0, time.UTC)))
	assert.False(t, isBusinessHour(time.Date(2026, 3, 16, 8, 59, 0, 0, time.UTC)))
	assert.False(t, isBusinessHour(time.Date(2026, 3, 21, 10, 0, 0, 0, time.UTC)))
}
