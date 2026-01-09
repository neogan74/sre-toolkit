// Package analyzer provides frequency and pattern analysis for Prometheus alerts.
package analyzer

import (
	"sort"
	"time"

	"github.com/neogan/sre-toolkit/internal/alert-analyzer/collector"
)

// FrequencyResult represents the firing frequency analysis for a single alert
type FrequencyResult struct {
	AlertName   string        `json:"alert_name"`
	FiringCount int           `json:"firing_count"`
	TotalTime   time.Duration `json:"total_time"`
	AvgDuration time.Duration `json:"avg_duration"`
	LastFired   time.Time     `json:"last_fired"`
	Severity    string        `json:"severity"`
}

// FrequencyAnalyzer analyzes alert firing frequency
type FrequencyAnalyzer struct {
	history *collector.AlertHistory
}

// NewFrequencyAnalyzer creates a new frequency analyzer
func NewFrequencyAnalyzer(history *collector.AlertHistory) *FrequencyAnalyzer {
	return &FrequencyAnalyzer{
		history: history,
	}
}

// Analyze performs frequency analysis and returns results for all alerts
func (a *FrequencyAnalyzer) Analyze() []FrequencyResult {
	// Group alerts by name
	grouped := collector.GroupAlertsByName(a.history.Alerts)

	results := make([]FrequencyResult, 0, len(grouped))

	for alertName, alerts := range grouped {
		result := a.analyzeAlertGroup(alertName, alerts)
		results = append(results, result)
	}

	// Sort by firing count descending
	sort.Slice(results, func(i, j int) bool {
		return results[i].FiringCount > results[j].FiringCount
	})

	return results
}

// AnalyzeTopN returns the top N most frequently firing alerts
func (a *FrequencyAnalyzer) AnalyzeTopN(n int) []FrequencyResult {
	allResults := a.Analyze()

	if n > len(allResults) {
		n = len(allResults)
	}

	return allResults[:n]
}

// analyzeAlertGroup analyzes a group of alerts with the same name
func (a *FrequencyAnalyzer) analyzeAlertGroup(alertName string, alerts []collector.Alert) FrequencyResult {
	firingCount := 0
	var totalTime time.Duration
	var lastFired time.Time
	var severity string

	for _, alert := range alerts {
		// Count each alert instance as a firing
		firingCount++

		// Accumulate total time in firing state
		totalTime += alert.Duration()

		// Track the most recent firing time
		if alert.FiredAt.After(lastFired) {
			lastFired = alert.FiredAt
		}

		// Get severity from the alert (all should have the same severity for a given alert name)
		if severity == "" {
			severity = alert.GetSeverity()
		}
	}

	// Calculate average duration
	var avgDuration time.Duration
	if firingCount > 0 {
		avgDuration = totalTime / time.Duration(firingCount)
	}

	return FrequencyResult{
		AlertName:   alertName,
		FiringCount: firingCount,
		TotalTime:   totalTime,
		AvgDuration: avgDuration,
		LastFired:   lastFired,
		Severity:    severity,
	}
}

// GetNoisyAlerts returns alerts that fire frequently but resolve quickly (potential noise)
// Threshold: alerts that fire more than 'minFirings' times with avg duration less than 'maxDuration'
func (a *FrequencyAnalyzer) GetNoisyAlerts(minFirings int, maxDuration time.Duration) []FrequencyResult {
	allResults := a.Analyze()

	noisy := make([]FrequencyResult, 0)

	for _, result := range allResults {
		if result.FiringCount >= minFirings && result.AvgDuration <= maxDuration {
			noisy = append(noisy, result)
		}
	}

	// Sort by firing count descending
	sort.Slice(noisy, func(i, j int) bool {
		return noisy[i].FiringCount > noisy[j].FiringCount
	})

	return noisy
}

// SummaryStats holds summary statistics for all alerts
type SummaryStats struct {
	TotalAlerts        int           `json:"total_alerts"`
	UniqueAlerts       int           `json:"unique_alerts"`
	TotalFirings       int           `json:"total_firings"`
	AvgDuration        time.Duration `json:"avg_duration"`
	TotalFiringTime    time.Duration `json:"total_firing_time"`
	MostFrequent       string        `json:"most_frequent"`
	LongestAvgDuration string        `json:"longest_avg_duration"`
}

// GetSummaryStats returns overall summary statistics
func (a *FrequencyAnalyzer) GetSummaryStats() SummaryStats {
	results := a.Analyze()

	if len(results) == 0 {
		return SummaryStats{}
	}

	totalFirings := 0
	var totalFiringTime time.Duration

	for _, result := range results {
		totalFirings += result.FiringCount
		totalFiringTime += result.TotalTime
	}

	avgDuration := time.Duration(0)
	if totalFirings > 0 {
		avgDuration = totalFiringTime / time.Duration(totalFirings)
	}

	// Find alert with longest average duration
	longestAvgDuration := results[0].AlertName
	maxAvgDuration := results[0].AvgDuration
	for _, result := range results {
		if result.AvgDuration > maxAvgDuration {
			maxAvgDuration = result.AvgDuration
			longestAvgDuration = result.AlertName
		}
	}

	return SummaryStats{
		TotalAlerts:        a.history.CountAlerts(),
		UniqueAlerts:       len(results),
		TotalFirings:       totalFirings,
		AvgDuration:        avgDuration,
		TotalFiringTime:    totalFiringTime,
		MostFrequent:       results[0].AlertName, // Already sorted by firing count
		LongestAvgDuration: longestAvgDuration,
	}
}
