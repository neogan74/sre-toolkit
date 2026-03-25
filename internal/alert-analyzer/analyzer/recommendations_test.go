package analyzer

import (
	"testing"
	"time"

	"github.com/neogan/sre-toolkit/internal/alert-analyzer/collector"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestRecommendationEngine_Generate(t *testing.T) {
	engine := NewRecommendationEngine()

	frequency := []FrequencyResult{
		{
			AlertName:   "CriticalNoisyAlert",
			FiringCount: 32,
			AvgDuration: 3 * time.Minute,
			Severity:    "critical",
		},
		{
			AlertName:   "StableInfoAlert",
			FiringCount: 2,
			AvgDuration: 40 * time.Minute,
			Severity:    "info",
		},
	}

	flapping := []FlappingResult{
		{
			AlertName:       "CriticalNoisyAlert",
			TransitionCount: 12,
			FlappingScore:   6.0,
			IsFlapping:      true,
			Severity:        "critical",
		},
	}

	correlation := []CorrelationResult{
		{
			AlertA:            "DatabaseConnectionFlap",
			AlertB:            "APILatencyHigh",
			CoOccurrenceCount: 6,
			CorrelationScore:  0.83,
		},
		{
			AlertA:            "WeakA",
			AlertB:            "WeakB",
			CoOccurrenceCount: 1,
			CorrelationScore:  0.25,
		},
	}

	rules := []collector.AlertRule{
		{
			Name:   "NeverFiresAlert",
			Labels: map[string]string{"severity": "warning"},
		},
		{
			Name:      "BrokenRuleAlert",
			Cluster:   "prod",
			Labels:    map[string]string{"severity": "critical"},
			LastError: "parse error: unexpected identifier",
		},
	}

	recommendations := engine.Generate(frequency, flapping, correlation, rules)
	require.Len(t, recommendations, 6)

	assert.Equal(t, RecommendationPriorityCritical, recommendations[0].Priority)
	assert.Equal(t, RecommendationCategoryDeadRule, recommendations[0].Category)
	assert.Equal(t, "BrokenRuleAlert [prod]", recommendations[0].Target)

	assert.Equal(t, RecommendationPriorityCritical, recommendations[1].Priority)
	assert.Equal(t, RecommendationCategoryReview, recommendations[1].Category)
	assert.Equal(t, "CriticalNoisyAlert", recommendations[1].Target)

	assert.Equal(t, RecommendationCategoryStability, recommendations[2].Category)
	assert.Equal(t, "CriticalNoisyAlert", recommendations[2].Target)

	assert.Equal(t, RecommendationCategoryTuning, recommendations[3].Category)
	assert.Equal(t, SignalToNoiseLow, recommendations[3].SignalToNoise)

	assert.Equal(t, RecommendationCategoryDeadRule, recommendations[4].Category)
	assert.Equal(t, "NeverFiresAlert", recommendations[4].Target)

	assert.Equal(t, RecommendationCategoryDeduplication, recommendations[5].Category)
	assert.Equal(t, []string{"DatabaseConnectionFlap", "APILatencyHigh"}, recommendations[5].RelatedAlerts)
}

func TestAssessSignalToNoise(t *testing.T) {
	tests := []struct {
		name     string
		result   FrequencyResult
		expected string
	}{
		{
			name: "Low",
			result: FrequencyResult{
				FiringCount: 20,
				AvgDuration: 4 * time.Minute,
			},
			expected: SignalToNoiseLow,
		},
		{
			name: "Medium",
			result: FrequencyResult{
				FiringCount: 7,
				AvgDuration: 10 * time.Minute,
			},
			expected: SignalToNoiseMedium,
		},
		{
			name: "High",
			result: FrequencyResult{
				FiringCount: 2,
				AvgDuration: 30 * time.Minute,
			},
			expected: SignalToNoiseHigh,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.expected, assessSignalToNoise(tt.result))
		})
	}
}

func TestShouldPrioritizeReview(t *testing.T) {
	assert.True(t, shouldPrioritizeReview(
		FrequencyResult{Severity: "critical", FiringCount: 30},
		FlappingResult{},
	))

	assert.True(t, shouldPrioritizeReview(
		FrequencyResult{Severity: "warning", FiringCount: 55},
		FlappingResult{},
	))

	assert.False(t, shouldPrioritizeReview(
		FrequencyResult{Severity: "warning", FiringCount: 5},
		FlappingResult{},
	))
}
