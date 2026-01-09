package collector

import (
	"context"
	"fmt"
	"time"

	v1 "github.com/prometheus/client_golang/api/prometheus/v1"
	"github.com/prometheus/common/model"
	"github.com/rs/zerolog"

	"github.com/neogan/sre-toolkit/pkg/prometheus"
)

// PrometheusCollector collects alert data from Prometheus
type PrometheusCollector struct {
	client *prometheus.Client
	logger *zerolog.Logger
}

// NewPrometheusCollector creates a new Prometheus collector
func NewPrometheusCollector(client *prometheus.Client, logger *zerolog.Logger) *PrometheusCollector {
	return &PrometheusCollector{
		client: client,
		logger: logger,
	}
}

// Collect fetches alert history from Prometheus for the specified time range
func (c *PrometheusCollector) Collect(ctx context.Context, lookback time.Duration, resolution time.Duration) (*AlertHistory, error) {
	endTime := time.Now()
	startTime := endTime.Add(-lookback)

	c.logger.Info().
		Time("start", startTime).
		Time("end", endTime).
		Dur("lookback", lookback).
		Dur("resolution", resolution).
		Msg("Collecting alert data from Prometheus")

	// Query for all alerts in the time range
	// ALERTS{} returns all alert instances with their state
	query := "ALERTS{}"

	// Execute range query
	r := v1.Range{
		Start: startTime,
		End:   endTime,
		Step:  resolution,
	}

	result, err := c.client.QueryRange(ctx, query, r)
	if err != nil {
		return nil, fmt.Errorf("failed to query alerts: %w", err)
	}

	// Parse the result into Alert structs
	alerts, err := c.parseAlerts(result, startTime, endTime)
	if err != nil {
		return nil, fmt.Errorf("failed to parse alerts: %w", err)
	}

	history := &AlertHistory{
		Alerts:    alerts,
		StartTime: startTime,
		EndTime:   endTime,
		Source:    "prometheus",
	}

	c.logger.Info().
		Int("total_alerts", len(alerts)).
		Int("unique_alerts", history.CountUniqueAlerts()).
		Msg("Successfully collected alert data")

	return history, nil
}

// parseAlerts converts Prometheus query result into Alert structs
func (c *PrometheusCollector) parseAlerts(value model.Value, startTime, endTime time.Time) ([]Alert, error) {
	matrix, ok := value.(model.Matrix)
	if !ok {
		return nil, fmt.Errorf("unexpected result type: %s", value.Type())
	}

	// Map to track alert instances by unique key
	alertMap := make(map[string]*Alert)

	for _, stream := range matrix {
		// Extract alert name from metric labels
		alertName := string(stream.Metric["alertname"])
		if alertName == "" {
			c.logger.Warn().Msg("Skipping alert with no alertname")
			continue
		}

		// Extract other labels
		labels := make(map[string]string)
		annotations := make(map[string]string)

		for k, v := range stream.Metric {
			key := string(k)
			value := string(v)

			// Separate labels from annotations
			// Annotations typically have specific prefixes in Prometheus
			if key == "alertname" || key == "alertstate" {
				continue // Skip special labels
			}
			labels[key] = value
		}

		// Process each sample in the time series
		for _, sample := range stream.Values {
			timestamp := time.Unix(int64(sample.Timestamp)/1000, 0)
			value := float64(sample.Value)

			// Create unique key for this alert instance
			// Use alertname + label fingerprint
			key := createAlertKey(alertName, labels)

			// Check if we already have this alert instance
			alert, exists := alertMap[key]
			if !exists {
				// Create new alert instance
				alert = &Alert{
					Name:        alertName,
					Labels:      labels,
					Annotations: annotations,
					State:       "firing",
					Value:       value,
					ActiveAt:    timestamp,
					FiredAt:     timestamp,
				}
				alertMap[key] = alert
			} else {
				// Update existing alert
				// If value is 0, the alert was resolved
				if value == 0 && alert.ResolvedAt == nil {
					resolvedAt := timestamp
					alert.ResolvedAt = &resolvedAt
					alert.State = "inactive"
				}
			}
		}
	}

	// Convert map to slice
	alerts := make([]Alert, 0, len(alertMap))
	for _, alert := range alertMap {
		alerts = append(alerts, *alert)
	}

	return alerts, nil
}

// createAlertKey creates a unique key for an alert instance
func createAlertKey(name string, labels map[string]string) string {
	// Simple approach: concatenate name with sorted labels
	// For production, consider using a hash function
	key := name
	for k, v := range labels {
		key += fmt.Sprintf("_%s=%s", k, v)
	}
	return key
}

// CollectCurrentAlerts fetches only currently firing alerts
func (c *PrometheusCollector) CollectCurrentAlerts(ctx context.Context) ([]Alert, error) {
	c.logger.Info().Msg("Collecting currently firing alerts from Prometheus")

	// Query for currently firing alerts
	query := "ALERTS{alertstate=\"firing\"}"

	result, err := c.client.Query(ctx, query, time.Now())
	if err != nil {
		return nil, fmt.Errorf("failed to query current alerts: %w", err)
	}

	// Parse instant query result
	vector, ok := result.(model.Vector)
	if !ok {
		return nil, fmt.Errorf("unexpected result type: %s", result.Type())
	}

	alerts := make([]Alert, 0, len(vector))
	now := time.Now()

	for _, sample := range vector {
		alertName := string(sample.Metric["alertname"])
		if alertName == "" {
			continue
		}

		labels := make(map[string]string)
		for k, v := range sample.Metric {
			key := string(k)
			if key != "alertname" && key != "alertstate" {
				labels[key] = string(v)
			}
		}

		alert := Alert{
			Name:     alertName,
			Labels:   labels,
			State:    "firing",
			Value:    float64(sample.Value),
			ActiveAt: now,
			FiredAt:  now,
		}

		alerts = append(alerts, alert)
	}

	c.logger.Info().
		Int("firing_alerts", len(alerts)).
		Msg("Collected currently firing alerts")

	return alerts, nil
}
