// Package analyzer provides frequency and pattern analysis for Prometheus alerts.
package analyzer

import (
	"sort"
	"time"

	"github.com/neogan/sre-toolkit/internal/alert-analyzer/collector"
)

// CorrelationResult represents the co-occurrence analysis between two alert groups.
type CorrelationResult struct {
	AlertA            string        `json:"alert_a"`
	AlertB            string        `json:"alert_b"`
	CoOccurrenceCount int           `json:"co_occurrence_count"`
	CoverageA         float64       `json:"coverage_a"`
	CoverageB         float64       `json:"coverage_b"`
	CorrelationScore  float64       `json:"correlation_score"`
	AvgOverlap        time.Duration `json:"avg_overlap"`
	TotalOverlap      time.Duration `json:"total_overlap"`
}

// CorrelationAnalyzer analyzes which alerts tend to fire together.
type CorrelationAnalyzer struct {
	history *collector.AlertHistory
}

// NewCorrelationAnalyzer creates a new correlation analyzer.
func NewCorrelationAnalyzer(history *collector.AlertHistory) *CorrelationAnalyzer {
	return &CorrelationAnalyzer{history: history}
}

// Analyze calculates overlapping alert pairs and returns them ordered by correlation strength.
func (a *CorrelationAnalyzer) Analyze() []CorrelationResult {
	if a.history == nil || len(a.history.Alerts) == 0 {
		return []CorrelationResult{}
	}

	grouped := collector.GroupAlertsByName(a.history.Alerts)
	keys := make([]string, 0, len(grouped))
	for key := range grouped {
		keys = append(keys, key)
	}
	sort.Strings(keys)

	results := make([]CorrelationResult, 0)
	for i := 0; i < len(keys); i++ {
		for j := i + 1; j < len(keys); j++ {
			result := a.analyzePair(keys[i], grouped[keys[i]], keys[j], grouped[keys[j]])
			if result.CoOccurrenceCount > 0 {
				results = append(results, result)
			}
		}
	}

	sort.Slice(results, func(i, j int) bool {
		if results[i].CorrelationScore == results[j].CorrelationScore {
			if results[i].CoOccurrenceCount == results[j].CoOccurrenceCount {
				if results[i].TotalOverlap == results[j].TotalOverlap {
					if results[i].AlertA == results[j].AlertA {
						return results[i].AlertB < results[j].AlertB
					}
					return results[i].AlertA < results[j].AlertA
				}
				return results[i].TotalOverlap > results[j].TotalOverlap
			}
			return results[i].CoOccurrenceCount > results[j].CoOccurrenceCount
		}
		return results[i].CorrelationScore > results[j].CorrelationScore
	})

	return results
}

// AnalyzeTopN returns the strongest N correlated pairs.
func (a *CorrelationAnalyzer) AnalyzeTopN(n int) []CorrelationResult {
	results := a.Analyze()
	if n > len(results) {
		n = len(results)
	}
	return results[:n]
}

func (a *CorrelationAnalyzer) analyzePair(alertA string, alertsA []collector.Alert, alertB string, alertsB []collector.Alert) CorrelationResult {
	overlappedA := make([]bool, len(alertsA))
	overlappedB := make([]bool, len(alertsB))

	coOccurrenceCount := 0
	var totalOverlap time.Duration

	for i, left := range alertsA {
		leftEnd := a.alertEnd(left)
		for j, right := range alertsB {
			rightEnd := a.alertEnd(right)
			overlap := overlapDuration(left.FiredAt, leftEnd, right.FiredAt, rightEnd)
			if overlap <= 0 {
				continue
			}

			coOccurrenceCount++
			totalOverlap += overlap
			overlappedA[i] = true
			overlappedB[j] = true
		}
	}

	var avgOverlap time.Duration
	if coOccurrenceCount > 0 {
		avgOverlap = totalOverlap / time.Duration(coOccurrenceCount)
	}

	coverageA := coverageRatio(overlappedA)
	coverageB := coverageRatio(overlappedB)

	return CorrelationResult{
		AlertA:            alertA,
		AlertB:            alertB,
		CoOccurrenceCount: coOccurrenceCount,
		CoverageA:         coverageA,
		CoverageB:         coverageB,
		CorrelationScore:  (coverageA + coverageB) / 2,
		AvgOverlap:        avgOverlap,
		TotalOverlap:      totalOverlap,
	}
}

func (a *CorrelationAnalyzer) alertEnd(alert collector.Alert) time.Time {
	if alert.ResolvedAt != nil {
		return *alert.ResolvedAt
	}
	if !a.history.EndTime.IsZero() {
		return a.history.EndTime
	}
	return alert.FiredAt
}

func overlapDuration(startA, endA, startB, endB time.Time) time.Duration {
	if endA.Before(startA) || endB.Before(startB) {
		return 0
	}

	start := startA
	if startB.After(start) {
		start = startB
	}

	end := endA
	if endB.Before(end) {
		end = endB
	}

	if !end.After(start) {
		return 0
	}
	return end.Sub(start)
}

func coverageRatio(overlaps []bool) float64 {
	if len(overlaps) == 0 {
		return 0
	}

	count := 0
	for _, overlapped := range overlaps {
		if overlapped {
			count++
		}
	}

	return float64(count) / float64(len(overlaps))
}
