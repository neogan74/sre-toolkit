package analyzer

import (
	"testing"
	"time"

	"github.com/neogan/sre-toolkit/internal/alert-analyzer/collector"
)

func TestNewFlappingAnalyzer(t *testing.T) {
	history := &collector.AlertHistory{}

	tests := []struct {
		name              string
		threshold         float64
		expectedThreshold float64
	}{
		{
			name:              "default threshold when zero",
			threshold:         0,
			expectedThreshold: DefaultFlappingThreshold,
		},
		{
			name:              "default threshold when negative",
			threshold:         -1,
			expectedThreshold: DefaultFlappingThreshold,
		},
		{
			name:              "custom threshold",
			threshold:         5.0,
			expectedThreshold: 5.0,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			analyzer := NewFlappingAnalyzer(history, tt.threshold)
			if analyzer.threshold != tt.expectedThreshold {
				t.Errorf("NewFlappingAnalyzer() threshold = %v, want %v", analyzer.threshold, tt.expectedThreshold)
			}
		})
	}
}

func TestFlappingAnalyzer_Analyze(t *testing.T) {
	now := time.Now()

	tests := []struct {
		name           string
		alerts         []collector.Alert
		startTime      time.Time
		endTime        time.Time
		threshold      float64
		wantCount      int
		wantFlapping   int
		wantAlertNames []string
	}{
		{
			name:         "empty history",
			alerts:       []collector.Alert{},
			startTime:    now.Add(-time.Hour),
			endTime:      now,
			threshold:    3.0,
			wantCount:    0,
			wantFlapping: 0,
		},
		{
			name: "stable alert - no flapping",
			alerts: []collector.Alert{
				makeTestAlert("StableAlert", now.Add(-30*time.Minute), nil, "firing"),
			},
			startTime:    now.Add(-time.Hour),
			endTime:      now,
			threshold:    3.0,
			wantCount:    1,
			wantFlapping: 0,
		},
		{
			name: "flapping alert - multiple transitions",
			alerts: []collector.Alert{
				makeTestAlertWithResolved("FlappingAlert", now.Add(-50*time.Minute), now.Add(-45*time.Minute)),
				makeTestAlertWithResolved("FlappingAlert", now.Add(-40*time.Minute), now.Add(-35*time.Minute)),
				makeTestAlertWithResolved("FlappingAlert", now.Add(-30*time.Minute), now.Add(-25*time.Minute)),
				makeTestAlertWithResolved("FlappingAlert", now.Add(-20*time.Minute), now.Add(-15*time.Minute)),
			},
			startTime:    now.Add(-time.Hour),
			endTime:      now,
			threshold:    3.0,
			wantCount:    1,
			wantFlapping: 1, // 8 transitions in 1 hour = 8 transitions/hour
		},
		{
			name: "multiple alerts - mixed flapping",
			alerts: []collector.Alert{
				// StableAlert - only 1 firing
				makeTestAlert("StableAlert", now.Add(-30*time.Minute), nil, "firing"),
				// FlappingAlert - multiple transitions
				makeTestAlertWithResolved("FlappingAlert", now.Add(-50*time.Minute), now.Add(-45*time.Minute)),
				makeTestAlertWithResolved("FlappingAlert", now.Add(-40*time.Minute), now.Add(-35*time.Minute)),
				makeTestAlertWithResolved("FlappingAlert", now.Add(-30*time.Minute), now.Add(-25*time.Minute)),
			},
			startTime:    now.Add(-time.Hour),
			endTime:      now,
			threshold:    3.0,
			wantCount:    2,
			wantFlapping: 1,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			history := &collector.AlertHistory{
				Alerts:    tt.alerts,
				StartTime: tt.startTime,
				EndTime:   tt.endTime,
				Source:    "prometheus",
			}

			analyzer := NewFlappingAnalyzer(history, tt.threshold)
			results := analyzer.Analyze()

			if len(results) != tt.wantCount {
				t.Errorf("Analyze() returned %d results, want %d", len(results), tt.wantCount)
			}

			flappingCount := 0
			for _, r := range results {
				if r.IsFlapping {
					flappingCount++
				}
			}

			if flappingCount != tt.wantFlapping {
				t.Errorf("Analyze() found %d flapping alerts, want %d", flappingCount, tt.wantFlapping)
			}
		})
	}
}

func TestFlappingAnalyzer_AnalyzeTopN(t *testing.T) {
	now := time.Now()

	alerts := []collector.Alert{
		makeTestAlertWithResolved("Alert1", now.Add(-50*time.Minute), now.Add(-45*time.Minute)),
		makeTestAlertWithResolved("Alert2", now.Add(-50*time.Minute), now.Add(-45*time.Minute)),
		makeTestAlertWithResolved("Alert2", now.Add(-40*time.Minute), now.Add(-35*time.Minute)),
		makeTestAlertWithResolved("Alert3", now.Add(-50*time.Minute), now.Add(-45*time.Minute)),
		makeTestAlertWithResolved("Alert3", now.Add(-40*time.Minute), now.Add(-35*time.Minute)),
		makeTestAlertWithResolved("Alert3", now.Add(-30*time.Minute), now.Add(-25*time.Minute)),
	}

	history := &collector.AlertHistory{
		Alerts:    alerts,
		StartTime: now.Add(-time.Hour),
		EndTime:   now,
		Source:    "prometheus",
	}

	analyzer := NewFlappingAnalyzer(history, 3.0)

	tests := []struct {
		name    string
		n       int
		wantLen int
	}{
		{"top 1", 1, 1},
		{"top 2", 2, 2},
		{"top 10 (more than available)", 10, 3},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			results := analyzer.AnalyzeTopN(tt.n)
			if len(results) != tt.wantLen {
				t.Errorf("AnalyzeTopN(%d) returned %d results, want %d", tt.n, len(results), tt.wantLen)
			}
		})
	}
}

func TestFlappingAnalyzer_GetFlappingAlerts(t *testing.T) {
	now := time.Now()

	tests := []struct {
		name         string
		alerts       []collector.Alert
		threshold    float64
		wantFlapping int
	}{
		{
			name: "no flapping alerts",
			alerts: []collector.Alert{
				makeTestAlert("Stable", now.Add(-30*time.Minute), nil, "firing"),
			},
			threshold:    3.0,
			wantFlapping: 0,
		},
		{
			name: "one flapping alert",
			alerts: []collector.Alert{
				makeTestAlertWithResolved("Flapping", now.Add(-50*time.Minute), now.Add(-45*time.Minute)),
				makeTestAlertWithResolved("Flapping", now.Add(-40*time.Minute), now.Add(-35*time.Minute)),
				makeTestAlertWithResolved("Flapping", now.Add(-30*time.Minute), now.Add(-25*time.Minute)),
				makeTestAlertWithResolved("Flapping", now.Add(-20*time.Minute), now.Add(-15*time.Minute)),
			},
			threshold:    3.0,
			wantFlapping: 1,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			history := &collector.AlertHistory{
				Alerts:    tt.alerts,
				StartTime: now.Add(-time.Hour),
				EndTime:   now,
				Source:    "prometheus",
			}

			analyzer := NewFlappingAnalyzer(history, tt.threshold)
			results := analyzer.GetFlappingAlerts()

			if len(results) != tt.wantFlapping {
				t.Errorf("GetFlappingAlerts() returned %d results, want %d", len(results), tt.wantFlapping)
			}
		})
	}
}

func TestFlappingAnalyzer_GetSummary(t *testing.T) {
	now := time.Now()

	tests := []struct {
		name             string
		alerts           []collector.Alert
		threshold        float64
		wantTotalAlerts  int
		wantFlappingCnt  int
		wantMostFlapping string
	}{
		{
			name:             "empty history",
			alerts:           []collector.Alert{},
			threshold:        3.0,
			wantTotalAlerts:  0,
			wantFlappingCnt:  0,
			wantMostFlapping: "",
		},
		{
			name: "with flapping alert",
			alerts: []collector.Alert{
				makeTestAlert("Stable", now.Add(-30*time.Minute), nil, "firing"),
				makeTestAlertWithResolved("Flapping", now.Add(-50*time.Minute), now.Add(-45*time.Minute)),
				makeTestAlertWithResolved("Flapping", now.Add(-40*time.Minute), now.Add(-35*time.Minute)),
				makeTestAlertWithResolved("Flapping", now.Add(-30*time.Minute), now.Add(-25*time.Minute)),
				makeTestAlertWithResolved("Flapping", now.Add(-20*time.Minute), now.Add(-15*time.Minute)),
			},
			threshold:        3.0,
			wantTotalAlerts:  2,
			wantFlappingCnt:  1,
			wantMostFlapping: "Flapping",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			history := &collector.AlertHistory{
				Alerts:    tt.alerts,
				StartTime: now.Add(-time.Hour),
				EndTime:   now,
				Source:    "prometheus",
			}

			analyzer := NewFlappingAnalyzer(history, tt.threshold)
			summary := analyzer.GetSummary()

			if summary.TotalAlerts != tt.wantTotalAlerts {
				t.Errorf("GetSummary() TotalAlerts = %d, want %d", summary.TotalAlerts, tt.wantTotalAlerts)
			}
			if summary.FlappingAlerts != tt.wantFlappingCnt {
				t.Errorf("GetSummary() FlappingAlerts = %d, want %d", summary.FlappingAlerts, tt.wantFlappingCnt)
			}
			if summary.MostFlapping != tt.wantMostFlapping {
				t.Errorf("GetSummary() MostFlapping = %q, want %q", summary.MostFlapping, tt.wantMostFlapping)
			}
			if summary.Threshold != tt.threshold {
				t.Errorf("GetSummary() Threshold = %v, want %v", summary.Threshold, tt.threshold)
			}
		})
	}
}

func TestDetectStateTransitions(t *testing.T) {
	now := time.Now()

	tests := []struct {
		name           string
		alerts         []collector.Alert
		wantTransCount int
	}{
		{
			name:           "no alerts",
			alerts:         []collector.Alert{},
			wantTransCount: 0,
		},
		{
			name: "single firing - one transition",
			alerts: []collector.Alert{
				makeTestAlert("Alert1", now.Add(-30*time.Minute), nil, "firing"),
			},
			wantTransCount: 1, // inactive -> firing
		},
		{
			name: "firing then resolved - two transitions",
			alerts: []collector.Alert{
				makeTestAlertWithResolved("Alert1", now.Add(-30*time.Minute), now.Add(-15*time.Minute)),
			},
			wantTransCount: 2, // inactive -> firing, firing -> resolved
		},
		{
			name: "multiple fire-resolve cycles",
			alerts: []collector.Alert{
				makeTestAlertWithResolved("Alert1", now.Add(-50*time.Minute), now.Add(-45*time.Minute)),
				makeTestAlertWithResolved("Alert1", now.Add(-40*time.Minute), now.Add(-35*time.Minute)),
			},
			wantTransCount: 4, // 2 cycles = 4 transitions
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			history := &collector.AlertHistory{
				Alerts:    tt.alerts,
				StartTime: now.Add(-time.Hour),
				EndTime:   now,
				Source:    "prometheus",
			}
			analyzer := NewFlappingAnalyzer(history, 3.0)
			transitions := analyzer.detectStateTransitions(tt.alerts)

			if len(transitions) != tt.wantTransCount {
				t.Errorf("detectStateTransitions() returned %d transitions, want %d", len(transitions), tt.wantTransCount)
			}
		})
	}
}

// Helper functions for creating test alerts

func makeTestAlert(name string, firedAt time.Time, resolvedAt *time.Time, state string) collector.Alert {
	return collector.Alert{
		Name:       name,
		Labels:     map[string]string{"severity": "warning"},
		State:      state,
		FiredAt:    firedAt,
		ResolvedAt: resolvedAt,
	}
}

func makeTestAlertWithResolved(name string, firedAt, resolvedAt time.Time) collector.Alert {
	return collector.Alert{
		Name:       name,
		Labels:     map[string]string{"severity": "warning"},
		State:      "firing", // Alert was firing before being resolved
		FiredAt:    firedAt,
		ResolvedAt: &resolvedAt,
	}
}
