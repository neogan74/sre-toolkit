package collector

import (
	"context"
	"errors"
	"testing"
	"time"

	v1 "github.com/prometheus/client_golang/api/prometheus/v1"
	"github.com/prometheus/common/model"
	"github.com/rs/zerolog"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type fakeRulesClient struct {
	rulesFn func(ctx context.Context) (v1.RulesResult, error)
}

func (f *fakeRulesClient) Rules(ctx context.Context) (v1.RulesResult, error) {
	if f.rulesFn == nil {
		return v1.RulesResult{}, errors.New("unexpected Rules call")
	}
	return f.rulesFn(ctx)
}

func TestRuleCollector_CollectAlertRules(t *testing.T) {
	logger := zerolog.Nop()
	now := time.Date(2026, 3, 20, 10, 0, 0, 0, time.UTC)

	t.Run("Success", func(t *testing.T) {
		client := &fakeRulesClient{
			rulesFn: func(_ context.Context) (v1.RulesResult, error) {
				return v1.RulesResult{
					Groups: []v1.RuleGroup{
						{
							Name: "infrastructure",
							File: "/etc/prometheus/rules/infra.yaml",
							Rules: v1.Rules{
								v1.AlertingRule{
									Name:           "HighCPU",
									Query:          `sum(rate(cpu[5m])) > 0.9`,
									Duration:       300,
									Labels:         model.LabelSet{"severity": "critical"},
									Annotations:    model.LabelSet{"summary": "CPU high"},
									Health:         v1.RuleHealthGood,
									LastEvaluation: now,
									State:          "inactive",
								},
								v1.RecordingRule{
									Name: "job:http_requests:rate5m",
								},
							},
						},
					},
				}, nil
			},
		}

		collector := NewRuleCollector(client, &logger)
		rules, err := collector.CollectAlertRules(context.Background(), "prod")
		require.NoError(t, err)
		require.Len(t, rules, 1)

		assert.Equal(t, "HighCPU", rules[0].Name)
		assert.Equal(t, "prod", rules[0].Cluster)
		assert.Equal(t, "infrastructure", rules[0].Group)
		assert.Equal(t, 5*time.Minute, rules[0].Duration)
		assert.Equal(t, "critical", rules[0].Labels["severity"])
		assert.Equal(t, "CPU high", rules[0].Annotations["summary"])
		assert.Equal(t, "inactive", rules[0].State)
	})

	t.Run("API Error", func(t *testing.T) {
		client := &fakeRulesClient{
			rulesFn: func(_ context.Context) (v1.RulesResult, error) {
				return v1.RulesResult{}, errors.New("boom")
			},
		}

		collector := NewRuleCollector(client, &logger)
		_, err := collector.CollectAlertRules(context.Background(), "prod")
		require.Error(t, err)
		assert.ErrorContains(t, err, "boom")
	})
}
