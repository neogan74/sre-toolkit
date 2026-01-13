// Package analyzer provides frequency and pattern analysis for Prometheus alerts.
package analyzer

import (
	"sort"
	"time"

	"github.com/neogan/sre-toolkit/internal/alert-analyzer/collector"
)

// StateTransition represents a single state change event for an alert.
type StateTransition struct {
	AlertKey  string    `json:"alert_key"`
	Timestamp time.Time `json:"timestamp"`
	FromState string    `json:"from_state"`
	ToState   string    `json:"to_state"`
}

// FlappingResult represents the flapping analysis for a single alert.
type FlappingResult struct {
	AlertName        string        `json:"alert_name"`
	TransitionCount  int           `json:"transition_count"`
	FlappingScore    float64       `json:"flapping_score"`     // transitions per hour
	AvgStateDuration time.Duration `json:"avg_state_duration"` // average time in each state
	ShortestDuration time.Duration `json:"shortest_duration"`  // shortest interval between transitions
	IsFlapping       bool          `json:"is_flapping"`
	Severity         string        `json:"severity"`
}

// FlappingAnalyzer detects alerts that constantly switch between firing and resolved states.
type FlappingAnalyzer struct {
	history   *collector.AlertHistory
	threshold float64 // transitions per hour to be considered flapping
}

// DefaultFlappingThreshold is the default threshold for flapping detection (3 transitions/hour).
const DefaultFlappingThreshold = 3.0

// NewFlappingAnalyzer creates a new flapping analyzer with the specified threshold.
func NewFlappingAnalyzer(history *collector.AlertHistory, threshold float64) *FlappingAnalyzer {
	if threshold <= 0 {
		threshold = DefaultFlappingThreshold
	}
	return &FlappingAnalyzer{
		history:   history,
		threshold: threshold,
	}
}

// Analyze performs flapping analysis and returns results for all alerts.
func (a *FlappingAnalyzer) Analyze() []FlappingResult {
	if a.history == nil || len(a.history.Alerts) == 0 {
		return []FlappingResult{}
	}

	// Group alerts by name
	grouped := collector.GroupAlertsByName(a.history.Alerts)

	// Calculate the analysis period duration
	analysisPeriod := a.history.EndTime.Sub(a.history.StartTime)
	if analysisPeriod <= 0 {
		analysisPeriod = time.Hour // Default to 1 hour if period is invalid
	}

	results := make([]FlappingResult, 0, len(grouped))

	for alertName, alerts := range grouped {
		result := a.analyzeAlertGroup(alertName, alerts, analysisPeriod)
		results = append(results, result)
	}

	// Sort by flapping score descending
	sort.Slice(results, func(i, j int) bool {
		return results[i].FlappingScore > results[j].FlappingScore
	})

	return results
}

// AnalyzeTopN returns the top N most flapping alerts.
func (a *FlappingAnalyzer) AnalyzeTopN(n int) []FlappingResult {
	allResults := a.Analyze()

	if n > len(allResults) {
		n = len(allResults)
	}

	return allResults[:n]
}

// GetFlappingAlerts returns only alerts that are considered flapping (above threshold).
func (a *FlappingAnalyzer) GetFlappingAlerts() []FlappingResult {
	allResults := a.Analyze()

	flapping := make([]FlappingResult, 0)
	for _, result := range allResults {
		if result.IsFlapping {
			flapping = append(flapping, result)
		}
	}

	return flapping
}

// analyzeAlertGroup analyzes a group of alerts with the same name for flapping behavior.
func (a *FlappingAnalyzer) analyzeAlertGroup(alertName string, alerts []collector.Alert, analysisPeriod time.Duration) FlappingResult {
	if len(alerts) == 0 {
		return FlappingResult{AlertName: alertName}
	}

	// Sort alerts by fired time
	sortedAlerts := make([]collector.Alert, len(alerts))
	copy(sortedAlerts, alerts)
	sort.Slice(sortedAlerts, func(i, j int) bool {
		return sortedAlerts[i].FiredAt.Before(sortedAlerts[j].FiredAt)
	})

	// Detect state transitions
	transitions := a.detectStateTransitions(sortedAlerts)

	// Calculate flapping score (transitions per hour)
	hoursInPeriod := analysisPeriod.Hours()
	if hoursInPeriod <= 0 {
		hoursInPeriod = 1
	}
	flappingScore := float64(len(transitions)) / hoursInPeriod

	// Calculate average state duration and shortest duration
	avgDuration, shortestDuration := a.calculateDurations(sortedAlerts, transitions)

	// Get severity from first alert
	severity := "unknown"
	if len(sortedAlerts) > 0 {
		severity = sortedAlerts[0].GetSeverity()
	}

	return FlappingResult{
		AlertName:        alertName,
		TransitionCount:  len(transitions),
		FlappingScore:    flappingScore,
		AvgStateDuration: avgDuration,
		ShortestDuration: shortestDuration,
		IsFlapping:       flappingScore >= a.threshold,
		Severity:         severity,
	}
}

// detectStateTransitions identifies state changes in a sorted list of alerts.
// A transition occurs when an alert fires after being resolved, or resolves after firing.
func (a *FlappingAnalyzer) detectStateTransitions(sortedAlerts []collector.Alert) []StateTransition {
	if len(sortedAlerts) == 0 {
		return []StateTransition{}
	}

	transitions := make([]StateTransition, 0)

	// Track the current state
	currentState := "inactive"

	for _, alert := range sortedAlerts {
		// Alert started firing
		if currentState != "firing" && alert.State == "firing" {
			transitions = append(transitions, StateTransition{
				AlertKey:  alert.Name,
				Timestamp: alert.FiredAt,
				FromState: currentState,
				ToState:   "firing",
			})
			currentState = "firing"
		}

		// Alert was resolved
		if alert.ResolvedAt != nil && currentState == "firing" {
			transitions = append(transitions, StateTransition{
				AlertKey:  alert.Name,
				Timestamp: *alert.ResolvedAt,
				FromState: "firing",
				ToState:   "resolved",
			})
			currentState = "resolved"
		}
	}

	return transitions
}

// calculateDurations calculates the average and shortest duration between transitions.
func (a *FlappingAnalyzer) calculateDurations(alerts []collector.Alert, transitions []StateTransition) (avg, shortest time.Duration) {
	if len(transitions) < 2 {
		// With fewer than 2 transitions, calculate based on alert durations
		if len(alerts) > 0 {
			var totalDuration time.Duration
			for _, alert := range alerts {
				totalDuration += alert.Duration()
			}
			avg = totalDuration / time.Duration(len(alerts))
		}
		return avg, 0
	}

	// Calculate intervals between transitions
	var totalInterval time.Duration
	shortest = time.Duration(1<<63 - 1) // Max duration as starting point

	for i := 1; i < len(transitions); i++ {
		interval := transitions[i].Timestamp.Sub(transitions[i-1].Timestamp)
		totalInterval += interval

		if interval < shortest {
			shortest = interval
		}
	}

	avg = totalInterval / time.Duration(len(transitions)-1)

	return avg, shortest
}

// FlappingSummary holds summary statistics for flapping analysis.
type FlappingSummary struct {
	TotalAlerts      int     `json:"total_alerts"`
	FlappingAlerts   int     `json:"flapping_alerts"`
	AvgFlappingScore float64 `json:"avg_flapping_score"`
	MaxFlappingScore float64 `json:"max_flapping_score"`
	MostFlapping     string  `json:"most_flapping"`
	Threshold        float64 `json:"threshold"`
}

// GetSummary returns overall summary statistics for flapping analysis.
func (a *FlappingAnalyzer) GetSummary() FlappingSummary {
	results := a.Analyze()

	if len(results) == 0 {
		return FlappingSummary{Threshold: a.threshold}
	}

	flappingCount := 0
	var totalScore float64
	maxScore := 0.0
	mostFlapping := ""

	for _, result := range results {
		totalScore += result.FlappingScore

		if result.IsFlapping {
			flappingCount++
		}

		if result.FlappingScore > maxScore {
			maxScore = result.FlappingScore
			mostFlapping = result.AlertName
		}
	}

	avgScore := totalScore / float64(len(results))

	return FlappingSummary{
		TotalAlerts:      len(results),
		FlappingAlerts:   flappingCount,
		AvgFlappingScore: avgScore,
		MaxFlappingScore: maxScore,
		MostFlapping:     mostFlapping,
		Threshold:        a.threshold,
	}
}
