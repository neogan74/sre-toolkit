package collector

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

func TestAlert_Getters(t *testing.T) {
	alert := Alert{
		Name: "TestAlert",
		Labels: map[string]string{
			"severity":  "critical",
			"namespace": "monitoring",
			"service":   "prometheus",
		},
		Annotations: map[string]string{
			"summary": "High load",
		},
	}

	assert.Equal(t, "TestAlert", alert.GetAlertName())
	assert.Equal(t, "critical", alert.GetSeverity())
	assert.Equal(t, "monitoring", alert.GetNamespace())
	assert.Equal(t, "prometheus", alert.GetService())

	emptyAlert := Alert{Name: "EmptyAlert"}
	assert.Equal(t, "EmptyAlert", emptyAlert.GetAlertName())
	assert.Equal(t, "unknown", emptyAlert.GetSeverity())
	assert.Equal(t, "", emptyAlert.GetNamespace())
	assert.Equal(t, "", emptyAlert.GetService())
}

func TestAlert_Duration(t *testing.T) {
	now := time.Now()
	firedAt := now.Add(-10 * time.Minute)

	t.Run("Resolved Alert", func(t *testing.T) {
		resolvedAt := now.Add(-5 * time.Minute)
		alert := Alert{
			FiredAt:    firedAt,
			ResolvedAt: &resolvedAt,
		}
		assert.Equal(t, 5*time.Minute, alert.Duration())
		assert.True(t, alert.IsResolved())
	})

	t.Run("Firing Alert", func(t *testing.T) {
		alert := Alert{
			FiredAt: firedAt,
			State:   "firing",
		}
		// Duration should be close to 10 minutes
		assert.InDelta(t, 10*time.Minute, alert.Duration(), float64(time.Second))
		assert.False(t, alert.IsResolved())
		assert.True(t, alert.IsFiring())
	})
}

func TestAlertHistory_Counts(t *testing.T) {
	alerts := []Alert{
		{Name: "Alert1"},
		{Name: "Alert2"},
		{Name: "Alert1"},
	}

	history := AlertHistory{
		Alerts: alerts,
	}

	assert.Equal(t, 3, history.CountAlerts())
	assert.Equal(t, 2, history.CountUniqueAlerts())

	names := history.GetAlertNames()
	assert.Len(t, names, 2)
	assert.Contains(t, names, "Alert1")
	assert.Contains(t, names, "Alert2")
}

func TestGroupAlertsByName(t *testing.T) {
	alerts := []Alert{
		{Name: "Alert1", Cluster: "cluster1", Labels: map[string]string{"instance": "1"}},
		{Name: "Alert1", Cluster: "cluster2", Labels: map[string]string{"instance": "1"}},
		{Name: "Alert2", Labels: map[string]string{"instance": "1"}},
	}

	groups := GroupAlertsByName(alerts)

	assert.Len(t, groups, 3)
	assert.Len(t, groups["Alert1 [cluster1]"], 1)
	assert.Len(t, groups["Alert1 [cluster2]"], 1)
	assert.Len(t, groups["Alert2"], 1)
}

func TestAlertRule_GetGroupingKey(t *testing.T) {
	t.Run("with cluster", func(t *testing.T) {
		rule := AlertRule{Name: "HighCPU", Cluster: "prod"}
		assert.Equal(t, "HighCPU [prod]", rule.GetGroupingKey())
	})
	t.Run("without cluster", func(t *testing.T) {
		rule := AlertRule{Name: "HighCPU"}
		assert.Equal(t, "HighCPU", rule.GetGroupingKey())
	})
}

func TestAlertRule_GetSeverity(t *testing.T) {
	t.Run("with severity label", func(t *testing.T) {
		rule := AlertRule{Labels: map[string]string{"severity": "critical"}}
		assert.Equal(t, "critical", rule.GetSeverity())
	})
	t.Run("without severity label", func(t *testing.T) {
		rule := AlertRule{Labels: map[string]string{}}
		assert.Equal(t, "unknown", rule.GetSeverity())
	})
	t.Run("nil labels", func(t *testing.T) {
		rule := AlertRule{}
		assert.Equal(t, "unknown", rule.GetSeverity())
	})
}

func TestAlertHistory_Merge(t *testing.T) {
	history1 := &AlertHistory{
		Alerts:    []Alert{{Name: "Alert1"}},
		StartTime: time.Now().Add(-2 * time.Hour),
		EndTime:   time.Now().Add(-1 * time.Hour),
		Source:    "prometheus1",
	}

	history2 := &AlertHistory{
		Alerts:    []Alert{{Name: "Alert2"}},
		StartTime: time.Now().Add(-3 * time.Hour), // earlier start
		EndTime:   time.Now(),                     // later end
		Source:    "prometheus2",
	}

	history1.Merge(history2)

	assert.Len(t, history1.Alerts, 2)
	assert.Equal(t, history2.StartTime, history1.StartTime)
	assert.Equal(t, history2.EndTime, history1.EndTime)
	assert.Equal(t, "prometheus1, prometheus2", history1.Source)
}
