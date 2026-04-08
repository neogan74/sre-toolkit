package analyzer

import (
	"testing"
	"time"

	"github.com/neogan/sre-toolkit/internal/alert-analyzer/collector"
	"github.com/stretchr/testify/assert"
)

func TestCorrelationAnalyzer_Analyze(t *testing.T) {
	now := time.Now()
	history := &collector.AlertHistory{
		StartTime: now.Add(-2 * time.Hour),
		EndTime:   now,
		Alerts: []collector.Alert{
			{
				Name:       "AlertA",
				FiredAt:    now.Add(-80 * time.Minute),
				ResolvedAt: TimePtr(now.Add(-60 * time.Minute)),
			},
			{
				Name:       "AlertA",
				FiredAt:    now.Add(-40 * time.Minute),
				ResolvedAt: TimePtr(now.Add(-25 * time.Minute)),
			},
			{
				Name:       "AlertB",
				FiredAt:    now.Add(-70 * time.Minute),
				ResolvedAt: TimePtr(now.Add(-55 * time.Minute)),
			},
			{
				Name:       "AlertB",
				FiredAt:    now.Add(-38 * time.Minute),
				ResolvedAt: TimePtr(now.Add(-10 * time.Minute)),
			},
			{
				Name:       "AlertC",
				FiredAt:    now.Add(-15 * time.Minute),
				ResolvedAt: TimePtr(now.Add(-5 * time.Minute)),
			},
		},
	}

	analyzer := NewCorrelationAnalyzer(history)
	results := analyzer.Analyze()

	assert.Len(t, results, 2)
	assert.Equal(t, "AlertA", results[0].AlertA)
	assert.Equal(t, "AlertB", results[0].AlertB)
	assert.Equal(t, 2, results[0].CoOccurrenceCount)
	assert.InDelta(t, 1.0, results[0].CoverageA, 0.001)
	assert.InDelta(t, 1.0, results[0].CoverageB, 0.001)
	assert.InDelta(t, 1.0, results[0].CorrelationScore, 0.001)
	assert.Equal(t, 23*time.Minute, results[0].TotalOverlap)
	assert.Equal(t, 11*time.Minute+30*time.Second, results[0].AvgOverlap)

	assert.Equal(t, "AlertB", results[1].AlertA)
	assert.Equal(t, "AlertC", results[1].AlertB)
	assert.Equal(t, 1, results[1].CoOccurrenceCount)
	assert.InDelta(t, 0.5, results[1].CoverageA, 0.001)
	assert.InDelta(t, 1.0, results[1].CoverageB, 0.001)
	assert.InDelta(t, 0.75, results[1].CorrelationScore, 0.001)
}

func TestCorrelationAnalyzer_AnalyzeTopN(t *testing.T) {
	now := time.Now()
	history := &collector.AlertHistory{
		StartTime: now.Add(-time.Hour),
		EndTime:   now,
		Alerts: []collector.Alert{
			{Name: "AlertA", FiredAt: now.Add(-50 * time.Minute), ResolvedAt: TimePtr(now.Add(-20 * time.Minute))},
			{Name: "AlertB", FiredAt: now.Add(-45 * time.Minute), ResolvedAt: TimePtr(now.Add(-15 * time.Minute))},
			{Name: "AlertC", FiredAt: now.Add(-10 * time.Minute), ResolvedAt: TimePtr(now.Add(-5 * time.Minute))},
		},
	}

	analyzer := NewCorrelationAnalyzer(history)
	results := analyzer.AnalyzeTopN(1)

	assert.Len(t, results, 1)
	assert.Equal(t, "AlertA", results[0].AlertA)
	assert.Equal(t, "AlertB", results[0].AlertB)
}

func TestCorrelationAnalyzer_Analyze_NilHistory(t *testing.T) {
	a := NewCorrelationAnalyzer(nil)
	assert.Empty(t, a.Analyze())
}

func TestCorrelationAnalyzer_Analyze_EmptyAlerts(t *testing.T) {
	a := NewCorrelationAnalyzer(&collector.AlertHistory{})
	assert.Empty(t, a.Analyze())
}

func TestCorrelationAnalyzer_AnalyzeTopN_ZeroN(t *testing.T) {
	now := time.Now()
	history := &collector.AlertHistory{
		Alerts: []collector.Alert{
			{Name: "AlertA", FiredAt: now.Add(-50 * time.Minute), ResolvedAt: TimePtr(now.Add(-20 * time.Minute))},
			{Name: "AlertB", FiredAt: now.Add(-45 * time.Minute), ResolvedAt: TimePtr(now.Add(-15 * time.Minute))},
		},
	}
	a := NewCorrelationAnalyzer(history)
	assert.Empty(t, a.AnalyzeTopN(0))
}

func TestAlertEnd_FallbackBranches(t *testing.T) {
	now := time.Now()

	t.Run("uses history EndTime when alert not resolved", func(t *testing.T) {
		endTime := now.Add(-5 * time.Minute)
		history := &collector.AlertHistory{EndTime: endTime}
		a := NewCorrelationAnalyzer(history)
		alert := collector.Alert{FiredAt: now.Add(-30 * time.Minute)} // no ResolvedAt
		assert.Equal(t, endTime, a.alertEnd(alert))
	})

	t.Run("falls back to FiredAt when EndTime is zero", func(t *testing.T) {
		history := &collector.AlertHistory{} // zero EndTime
		a := NewCorrelationAnalyzer(history)
		firedAt := now.Add(-10 * time.Minute)
		alert := collector.Alert{FiredAt: firedAt}
		assert.Equal(t, firedAt, a.alertEnd(alert))
	})
}

func TestOverlapDuration(t *testing.T) {
	now := time.Now()

	assert.Equal(t, 10*time.Minute,
		overlapDuration(now, now.Add(20*time.Minute), now.Add(10*time.Minute), now.Add(30*time.Minute)))
	assert.Equal(t, time.Duration(0),
		overlapDuration(now, now.Add(5*time.Minute), now.Add(5*time.Minute), now.Add(10*time.Minute)))
	assert.Equal(t, time.Duration(0),
		overlapDuration(now.Add(10*time.Minute), now, now.Add(2*time.Minute), now.Add(5*time.Minute)))
}
