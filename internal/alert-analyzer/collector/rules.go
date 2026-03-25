package collector

import (
	"context"
	"time"

	v1 "github.com/prometheus/client_golang/api/prometheus/v1"
	"github.com/rs/zerolog"
)

type prometheusRulesAPI interface {
	Rules(ctx context.Context) (v1.RulesResult, error)
}

// RuleCollector discovers configured alerting rules from Prometheus.
type RuleCollector struct {
	client prometheusRulesAPI
	logger *zerolog.Logger
}

// NewRuleCollector creates a rule collector backed by a Prometheus client.
func NewRuleCollector(client prometheusRulesAPI, logger *zerolog.Logger) *RuleCollector {
	return &RuleCollector{
		client: client,
		logger: logger,
	}
}

// CollectAlertRules returns all configured alerting rules for a cluster.
func (c *RuleCollector) CollectAlertRules(ctx context.Context, clusterName string) ([]AlertRule, error) {
	result, err := c.client.Rules(ctx)
	if err != nil {
		return nil, err
	}

	rules := make([]AlertRule, 0)
	for _, group := range result.Groups {
		for _, rule := range group.Rules {
			alertRule, ok := rule.(v1.AlertingRule)
			if !ok {
				continue
			}

			labels := make(map[string]string, len(alertRule.Labels))
			for k, v := range alertRule.Labels {
				labels[string(k)] = string(v)
			}

			annotations := make(map[string]string, len(alertRule.Annotations))
			for k, v := range alertRule.Annotations {
				annotations[string(k)] = string(v)
			}

			rules = append(rules, AlertRule{
				Name:           alertRule.Name,
				Cluster:        clusterName,
				Group:          group.Name,
				File:           group.File,
				Query:          alertRule.Query,
				Duration:       time.Duration(alertRule.Duration) * time.Second,
				Labels:         labels,
				Annotations:    annotations,
				Health:         string(alertRule.Health),
				LastError:      alertRule.LastError,
				LastEvaluation: alertRule.LastEvaluation,
				State:          alertRule.State,
			})
		}
	}

	c.logger.Info().
		Str("cluster", clusterName).
		Int("alert_rules", len(rules)).
		Msg("Collected alerting rules from Prometheus")

	return rules, nil
}
