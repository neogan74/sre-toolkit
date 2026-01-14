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
		{Name: "Alert1", Labels: map[string]string{"instance": "1"}},
		{Name: "Alert2", Labels: map[string]string{"instance": "1"}},
		{Name: "Alert1", Labels: map[string]string{"instance": "2"}},
	}

	groups := GroupAlertsByName(alerts)

	assert.Len(t, groups, 2)
	assert.Len(t, groups["Alert1"], 2)
	assert.Len(t, groups["Alert2"], 1)
}
