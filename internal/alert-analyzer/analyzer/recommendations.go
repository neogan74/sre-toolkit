// Package analyzer provides frequency and pattern analysis for Prometheus alerts.
package analyzer

import (
	"fmt"
	"sort"
	"strings"
	"time"
)

const (
	RecommendationPriorityCritical = "critical"
	RecommendationPriorityHigh     = "high"
	RecommendationPriorityMedium   = "medium"
	RecommendationPriorityLow      = "low"

	RecommendationCategoryTuning        = "tuning"
	RecommendationCategoryStability     = "stability"
	RecommendationCategoryDeduplication = "deduplication"
	RecommendationCategoryReview        = "review"

	SignalToNoiseLow    = "low"
	SignalToNoiseMedium = "medium"
	SignalToNoiseHigh   = "high"
)

const (
	defaultNoisyAlertMinFirings      = 10
	defaultNoisyAlertMaxAvgDuration  = 10 * time.Minute
	defaultLowSignalAvgDuration      = 5 * time.Minute
	defaultMediumSignalAvgDuration   = 15 * time.Minute
	defaultStrongCorrelationScore    = 0.70
	defaultMinCorrelationOccurrences = 3
	defaultPriorityReviewMinFirings  = 25
)

// Recommendation describes a concrete follow-up action based on alert analysis.
type Recommendation struct {
	Category      string   `json:"category"`
	Priority      string   `json:"priority"`
	Target        string   `json:"target,omitempty"`
	RelatedAlerts []string `json:"related_alerts,omitempty"`
	SignalToNoise string   `json:"signal_to_noise,omitempty"`
	Summary       string   `json:"summary"`
	Action        string   `json:"action"`
}

// RecommendationEngine generates actionable guidance from alert analysis results.
type RecommendationEngine struct{}

// NewRecommendationEngine creates a new recommendation engine.
func NewRecommendationEngine() *RecommendationEngine {
	return &RecommendationEngine{}
}

// Generate builds recommendations from frequency, flapping, and correlation insights.
func (e *RecommendationEngine) Generate(frequency []FrequencyResult, flapping []FlappingResult, correlation []CorrelationResult) []Recommendation {
	recommendations := make([]Recommendation, 0)
	flappingByAlert := make(map[string]FlappingResult, len(flapping))

	for _, result := range flapping {
		flappingByAlert[result.AlertName] = result
		if result.IsFlapping {
			recommendations = append(recommendations, Recommendation{
				Category:      RecommendationCategoryStability,
				Priority:      recommendationPriorityForSeverity(result.Severity, RecommendationPriorityHigh),
				Target:        result.AlertName,
				SignalToNoise: SignalToNoiseLow,
				Summary:       fmt.Sprintf("%s changes state %d times (%.2f transitions/hour), which indicates flapping.", result.AlertName, result.TransitionCount, result.FlappingScore),
				Action:        "Increase `for:` duration or stabilize the underlying dependency before paging on this alert.",
			})
		}
	}

	for _, result := range frequency {
		signalToNoise := assessSignalToNoise(result)
		if result.FiringCount >= defaultNoisyAlertMinFirings && result.AvgDuration <= defaultNoisyAlertMaxAvgDuration {
			recommendations = append(recommendations, Recommendation{
				Category:      RecommendationCategoryTuning,
				Priority:      recommendationPriorityForSeverity(result.Severity, RecommendationPriorityMedium),
				Target:        result.AlertName,
				SignalToNoise: signalToNoise,
				Summary:       fmt.Sprintf("%s fired %d times with an average duration of %s.", result.AlertName, result.FiringCount, formatCompactDuration(result.AvgDuration)),
				Action:        "Review threshold sensitivity and increase `for:` to reduce short-lived noise.",
			})
		}

		if shouldPrioritizeReview(result, flappingByAlert[result.AlertName]) {
			recommendations = append(recommendations, Recommendation{
				Category:      RecommendationCategoryReview,
				Priority:      recommendationPriorityForSeverity(result.Severity, RecommendationPriorityHigh),
				Target:        result.AlertName,
				SignalToNoise: signalToNoise,
				Summary:       fmt.Sprintf("%s should be prioritized for rule review due to severity=%s and %d firings.", result.AlertName, result.Severity, result.FiringCount),
				Action:        "Review routing, owner, runbook quality, and whether the alert still deserves its current severity.",
			})
		}
	}

	for _, result := range correlation {
		if result.CorrelationScore < defaultStrongCorrelationScore || result.CoOccurrenceCount < defaultMinCorrelationOccurrences {
			continue
		}

		pair := []string{result.AlertA, result.AlertB}
		recommendations = append(recommendations, Recommendation{
			Category:      RecommendationCategoryDeduplication,
			Priority:      RecommendationPriorityMedium,
			Target:        strings.Join(pair, " + "),
			RelatedAlerts: pair,
			Summary:       fmt.Sprintf("%s and %s overlap %d times with a correlation score of %.2f.", result.AlertA, result.AlertB, result.CoOccurrenceCount, result.CorrelationScore),
			Action:        "Review grouping, inhibition, or runbook linkage so operators do not triage the same incident twice.",
		})
	}

	sortRecommendations(recommendations)
	return recommendations
}

func assessSignalToNoise(result FrequencyResult) string {
	switch {
	case result.FiringCount >= defaultNoisyAlertMinFirings && result.AvgDuration <= defaultLowSignalAvgDuration:
		return SignalToNoiseLow
	case result.FiringCount >= 5 && result.AvgDuration <= defaultMediumSignalAvgDuration:
		return SignalToNoiseMedium
	default:
		return SignalToNoiseHigh
	}
}

func shouldPrioritizeReview(freq FrequencyResult, flap FlappingResult) bool {
	if freq.Severity == RecommendationPriorityCritical && (freq.FiringCount >= defaultPriorityReviewMinFirings || flap.IsFlapping) {
		return true
	}

	return freq.FiringCount >= defaultPriorityReviewMinFirings*2
}

func recommendationPriorityForSeverity(severity string, fallback string) string {
	if severity == RecommendationPriorityCritical {
		return RecommendationPriorityCritical
	}
	return fallback
}

func sortRecommendations(recommendations []Recommendation) {
	sort.Slice(recommendations, func(i, j int) bool {
		left := recommendations[i]
		right := recommendations[j]
		if priorityWeight(left.Priority) != priorityWeight(right.Priority) {
			return priorityWeight(left.Priority) > priorityWeight(right.Priority)
		}
		if left.Category != right.Category {
			return left.Category < right.Category
		}
		return left.Target < right.Target
	})
}

func priorityWeight(priority string) int {
	switch priority {
	case RecommendationPriorityCritical:
		return 4
	case RecommendationPriorityHigh:
		return 3
	case RecommendationPriorityMedium:
		return 2
	default:
		return 1
	}
}

func formatCompactDuration(d time.Duration) string {
	if d == 0 {
		return "0s"
	}
	if d < time.Minute {
		return d.Truncate(time.Second).String()
	}
	if d < time.Hour {
		return d.Truncate(time.Minute).String()
	}
	return d.Truncate(time.Minute).String()
}
