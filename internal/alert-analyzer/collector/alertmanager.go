package collector

import (
	"context"
	"fmt"
	"time"

	"github.com/rs/zerolog"

	"github.com/neogan/sre-toolkit/pkg/alertmanager"
)

// AlertmanagerCollector collects alert data from Alertmanager
type AlertmanagerCollector struct {
	client *alertmanager.Client
	logger *zerolog.Logger
}

// NewAlertmanagerCollector creates a new Alertmanager collector
func NewAlertmanagerCollector(client *alertmanager.Client, logger *zerolog.Logger) *AlertmanagerCollector {
	return &AlertmanagerCollector{
		client: client,
		logger: logger,
	}
}

// CollectCurrentAlerts fetches currently firing alerts from Alertmanager
func (c *AlertmanagerCollector) CollectCurrentAlerts(ctx context.Context) (*AlertHistory, error) {
	c.logger.Info().Msg("Collecting firing alerts from Alertmanager")

	amAlerts, err := c.client.ListAlerts(ctx, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to list alerts: %w", err)
	}

	alerts := make([]Alert, 0, len(amAlerts))
	now := time.Now()

	for _, amAlert := range amAlerts {
		// Determine state
		// amAlert.Status usually contains state info, but top level response often lists active alerts
		// In AM v2 API, GET /alerts returns active alerts by default
		state := "firing"

		// Map Alertmanager alert to our Alert struct
		alert := Alert{
			Name:        amAlert.Labels["alertname"],
			Labels:      amAlert.Labels,
			Annotations: amAlert.Annotations,
			State:       state,
			Value:       1.0, // Active alerts count as 1
			ActiveAt:    amAlert.StartsAt,
			FiredAt:     amAlert.StartsAt,
		}

		if !amAlert.EndsAt.IsZero() && amAlert.EndsAt.Before(now) {
			alert.ResolvedAt = &amAlert.EndsAt
			alert.State = "resolved"
		}

		if alert.Name == "" {
			alert.Name = "unknown"
		}

		alerts = append(alerts, alert)
	}

	history := &AlertHistory{
		Alerts:    alerts,
		StartTime: now, // Snapshot time
		EndTime:   now,
		Source:    "alertmanager",
	}

	c.logger.Info().
		Int("total_alerts", len(alerts)).
		Msg("Successfully collected Alertmanager data")

	return history, nil
}
