package analyzer

import (
	"testing"
	"time"

	"github.com/neogan/sre-toolkit/internal/alert-analyzer/collector"
	"github.com/stretchr/testify/assert"
)

func TestFrequencyAnalyzer_Analyze(t *testing.T) {
	now := time.Now()
	alerts := []collector.Alert{
		{
			Name:       "AlertA",
			Labels:     map[string]string{"severity": "critical"},
			FiredAt:    now.Add(-10 * time.Minute),
			ResolvedAt: func() *time.Time { t := now.Add(-5 * time.Minute); return &t }(),
		},
		{
			Name:       "AlertA",
			Labels:     map[string]string{"severity": "critical"},
			FiredAt:    now.Add(-4 * time.Minute),
			ResolvedAt: func() *time.Time { t := now.Add(-1 * time.Minute); return &t }(),
		},
		{
			Name:       "AlertB",
			Labels:     map[string]string{"severity": "warning"},
			FiredAt:    now.Add(-20 * time.Minute),
			ResolvedAt: func() *time.Time { t := now.Add(-10 * time.Minute); return &t }(),
		},
	}

	history := &collector.AlertHistory{
		Alerts: alerts,
	}

	analyzer := NewFrequencyAnalyzer(history)
	results := analyzer.Analyze()

	assert.Len(t, results, 2)
	assert.Equal(t, "AlertA", results[0].AlertName) // Most frequent first
	assert.Equal(t, 2, results[0].FiringCount)
	assert.Equal(t, 8*time.Minute, results[0].TotalTime)
	assert.Equal(t, 4*time.Minute, results[0].AvgDuration)

	assert.Equal(t, "AlertB", results[1].AlertName)
	assert.Equal(t, 1, results[1].FiringCount)
}

func TestFrequencyAnalyzer_AnalyzeTopN(t *testing.T) {
	alerts := []collector.Alert{
		{Name: "AlertA"}, {Name: "AlertA"},
		{Name: "AlertB"},
		{Name: "AlertC"},
	}
	history := &collector.AlertHistory{Alerts: alerts}
	analyzer := NewFrequencyAnalyzer(history)

	results := analyzer.AnalyzeTopN(2)
	assert.Len(t, results, 2)
	assert.Equal(t, "AlertA", results[0].AlertName)
}

func TestFrequencyAnalyzer_GetNoisyAlerts(t *testing.T) {
	now := time.Now()
	// AlertA: 3 firings, short duration (noisy)
	// AlertB: 1 firing, long duration (not noisy)
	// AlertC: 1 firing, short duration (not enough firings)
	alerts := []collector.Alert{
		// AlertA - 3 times, 1 min each
		{Name: "AlertA", FiredAt: now.Add(-10 * time.Minute), ResolvedAt: TimePtr(now.Add(-9 * time.Minute))},
		{Name: "AlertA", FiredAt: now.Add(-8 * time.Minute), ResolvedAt: TimePtr(now.Add(-7 * time.Minute))},
		{Name: "AlertA", FiredAt: now.Add(-6 * time.Minute), ResolvedAt: TimePtr(now.Add(-5 * time.Minute))},

		// AlertB - 1 time, 60 min
		{Name: "AlertB", FiredAt: now.Add(-2 * time.Hour), ResolvedAt: TimePtr(now.Add(-1 * time.Hour))},

		// AlertC - 1 time, 1 min
		{Name: "AlertC", FiredAt: now.Add(-2 * time.Minute), ResolvedAt: TimePtr(now.Add(-1 * time.Minute))},
	}

	history := &collector.AlertHistory{Alerts: alerts}
	analyzer := NewFrequencyAnalyzer(history)

	noisy := analyzer.GetNoisyAlerts(2, 5*time.Minute)

	assert.Len(t, noisy, 1)
	assert.Equal(t, "AlertA", noisy[0].AlertName)
}

func TestFrequencyAnalyzer_GetSummaryStats(t *testing.T) {
	now := time.Now()
	alerts := []collector.Alert{
		{Name: "AlertA", FiredAt: now.Add(-10 * time.Minute), ResolvedAt: TimePtr(now.Add(-5 * time.Minute))},  // 5 min
		{Name: "AlertA", FiredAt: now.Add(-4 * time.Minute), ResolvedAt: TimePtr(now.Add(-2 * time.Minute))},   // 2 min
		{Name: "AlertB", FiredAt: now.Add(-20 * time.Minute), ResolvedAt: TimePtr(now.Add(-10 * time.Minute))}, // 10 min
	}
	// Total firings: 3
	// AlertA total: 7 min, avg: 3.5 min
	// AlertB total: 10 min, avg: 10 min
	// Total firing time: 17 min
	// Avg duration (global): 17 / 3 = 5.66 min
	// Longest avg duration: AlertB (10 min)

	history := &collector.AlertHistory{Alerts: alerts}
	analyzer := NewFrequencyAnalyzer(history)

	stats := analyzer.GetSummaryStats()

	assert.Equal(t, 3, stats.TotalFirings)
	assert.Equal(t, 2, stats.UniqueAlerts)
	assert.Equal(t, 17*time.Minute, stats.TotalFiringTime)
	assert.Equal(t, "AlertA", stats.MostFrequent)
	assert.Equal(t, "AlertB", stats.LongestAvgDuration)
}

func TimePtr(t time.Time) *time.Time {
	return &t
}
