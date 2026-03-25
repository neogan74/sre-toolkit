// Package analyzer provides frequency and pattern analysis for Prometheus alerts.
package analyzer

import (
	"sort"
	"time"

	"github.com/neogan/sre-toolkit/internal/alert-analyzer/collector"
)

var weekdayNames = []string{
	"Sunday",
	"Monday",
	"Tuesday",
	"Wednesday",
	"Thursday",
	"Friday",
	"Saturday",
}

// TemporalResult describes firing patterns over time for an alert.
type TemporalResult struct {
	AlertName           string  `json:"alert_name"`
	Severity            string  `json:"severity"`
	TotalFirings        int     `json:"total_firings"`
	PeakHour            int     `json:"peak_hour"`
	PeakHourCount       int     `json:"peak_hour_count"`
	PeakWeekday         string  `json:"peak_weekday"`
	PeakWeekdayCount    int     `json:"peak_weekday_count"`
	BusinessHoursRatio  float64 `json:"business_hours_ratio"`
	WeekendRatio        float64 `json:"weekend_ratio"`
	HourlyDistribution  []int   `json:"hourly_distribution"`
	WeekdayDistribution []int   `json:"weekday_distribution"`
}

// TemporalAnalyzer finds time-of-day and day-of-week patterns in alert firing history.
type TemporalAnalyzer struct {
	history *collector.AlertHistory
}

// NewTemporalAnalyzer creates a new temporal analyzer.
func NewTemporalAnalyzer(history *collector.AlertHistory) *TemporalAnalyzer {
	return &TemporalAnalyzer{history: history}
}

// Analyze returns temporal patterns for all alerts.
func (a *TemporalAnalyzer) Analyze() []TemporalResult {
	if a.history == nil || len(a.history.Alerts) == 0 {
		return []TemporalResult{}
	}

	grouped := collector.GroupAlertsByName(a.history.Alerts)
	results := make([]TemporalResult, 0, len(grouped))

	for alertName, alerts := range grouped {
		results = append(results, a.analyzeAlertGroup(alertName, alerts))
	}

	sort.Slice(results, func(i, j int) bool {
		if results[i].TotalFirings != results[j].TotalFirings {
			return results[i].TotalFirings > results[j].TotalFirings
		}
		return results[i].AlertName < results[j].AlertName
	})

	return results
}

// AnalyzeTopN returns the top N alerts with temporal pattern summaries.
func (a *TemporalAnalyzer) AnalyzeTopN(n int) []TemporalResult {
	all := a.Analyze()
	if n > len(all) {
		n = len(all)
	}
	return all[:n]
}

func (a *TemporalAnalyzer) analyzeAlertGroup(alertName string, alerts []collector.Alert) TemporalResult {
	hours := make([]int, 24)
	weekdays := make([]int, 7)
	businessHours := 0
	weekend := 0
	severity := "unknown"

	for _, alert := range alerts {
		firedAt := alert.FiredAt
		hours[firedAt.Hour()]++
		weekdays[int(firedAt.Weekday())]++

		if isBusinessHour(firedAt) {
			businessHours++
		}
		if firedAt.Weekday() == time.Saturday || firedAt.Weekday() == time.Sunday {
			weekend++
		}
		if severity == "unknown" {
			severity = alert.GetSeverity()
		}
	}

	peakHour, peakHourCount := maxBucket(hours)
	peakWeekdayIndex, peakWeekdayCount := maxBucket(weekdays)

	totalFirings := len(alerts)
	businessHoursRatio := 0.0
	weekendRatio := 0.0
	if totalFirings > 0 {
		businessHoursRatio = float64(businessHours) / float64(totalFirings)
		weekendRatio = float64(weekend) / float64(totalFirings)
	}

	return TemporalResult{
		AlertName:           alertName,
		Severity:            severity,
		TotalFirings:        totalFirings,
		PeakHour:            peakHour,
		PeakHourCount:       peakHourCount,
		PeakWeekday:         weekdayNames[peakWeekdayIndex],
		PeakWeekdayCount:    peakWeekdayCount,
		BusinessHoursRatio:  businessHoursRatio,
		WeekendRatio:        weekendRatio,
		HourlyDistribution:  hours,
		WeekdayDistribution: weekdays,
	}
}

func maxBucket(values []int) (index int, count int) {
	for i, value := range values {
		if value > count {
			index = i
			count = value
		}
	}
	return index, count
}

func isBusinessHour(ts time.Time) bool {
	if ts.Weekday() == time.Saturday || ts.Weekday() == time.Sunday {
		return false
	}
	hour := ts.Hour()
	return hour >= 9 && hour < 18
}
