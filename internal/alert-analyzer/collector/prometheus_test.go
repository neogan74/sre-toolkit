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

type fakePrometheusClient struct {
	queryFn      func(ctx context.Context, query string, ts time.Time) (model.Value, error)
	queryRangeFn func(ctx context.Context, query string, r v1.Range) (model.Value, error)
}

func (f *fakePrometheusClient) Query(ctx context.Context, query string, ts time.Time) (model.Value, error) {
	if f.queryFn == nil {
		return nil, errors.New("unexpected Query call")
	}
	return f.queryFn(ctx, query, ts)
}

func (f *fakePrometheusClient) QueryRange(ctx context.Context, query string, r v1.Range) (model.Value, error) {
	if f.queryRangeFn == nil {
		return nil, errors.New("unexpected QueryRange call")
	}
	return f.queryRangeFn(ctx, query, r)
}

func TestPrometheusCollector_Collect(t *testing.T) {
	logger := zerolog.Nop()
	start := model.TimeFromUnix(1735689600)

	t.Run("Success", func(t *testing.T) {
		client := &fakePrometheusClient{
			queryRangeFn: func(_ context.Context, query string, r v1.Range) (model.Value, error) {
				assert.Equal(t, "ALERTS{}", query)
				assert.Equal(t, time.Minute, r.Step)

				return model.Matrix{
					{
						Metric: model.Metric{
							"__name__":   "ALERTS",
							"alertname":  "HighCPU",
							"alertstate": "firing",
							"severity":   "critical",
							"namespace":  "prod",
						},
						Values: []model.SamplePair{
							{Timestamp: start, Value: 1},
							{Timestamp: start.Add(60 * time.Second), Value: 1},
							{Timestamp: start.Add(120 * time.Second), Value: 0},
						},
					},
				}, nil
			},
		}

		collector := NewPrometheusCollector(client, &logger)

		history, err := collector.Collect(context.Background(), "test-cluster", time.Hour, time.Minute)
		require.NoError(t, err)
		require.NotNil(t, history)
		require.Len(t, history.Alerts, 1)

		alert := history.Alerts[0]
		assert.Equal(t, "HighCPU", alert.Name)
		assert.Equal(t, "test-cluster", alert.Cluster)
		assert.Equal(t, "critical", alert.Labels["severity"])
		assert.Equal(t, "prod", alert.Labels["namespace"])
		assert.Equal(t, "inactive", alert.State)
		require.NotNil(t, alert.ResolvedAt)
		assert.True(t, alert.IsResolved())
		assert.Equal(t, alert.ActiveAt, alert.FiredAt)
	})

	t.Run("Empty Result", func(t *testing.T) {
		client := &fakePrometheusClient{
			queryRangeFn: func(_ context.Context, _ string, _ v1.Range) (model.Value, error) {
				return model.Matrix{}, nil
			},
		}

		collector := NewPrometheusCollector(client, &logger)

		history, err := collector.Collect(context.Background(), "test-cluster", time.Hour, time.Minute)
		require.NoError(t, err)
		assert.Equal(t, 0, history.CountAlerts())
	})

	t.Run("API Error", func(t *testing.T) {
		client := &fakePrometheusClient{
			queryRangeFn: func(_ context.Context, _ string, _ v1.Range) (model.Value, error) {
				return nil, errors.New("boom")
			},
		}

		collector := NewPrometheusCollector(client, &logger)

		_, err := collector.Collect(context.Background(), "test-cluster", time.Hour, time.Minute)
		require.Error(t, err)
		assert.ErrorContains(t, err, "failed to query alerts")
	})

	t.Run("Parse Error", func(t *testing.T) {
		client := &fakePrometheusClient{
			queryRangeFn: func(_ context.Context, _ string, _ v1.Range) (model.Value, error) {
				return model.Vector{}, nil
			},
		}

		collector := NewPrometheusCollector(client, &logger)

		_, err := collector.Collect(context.Background(), "test-cluster", time.Hour, time.Minute)
		require.Error(t, err)
		assert.ErrorContains(t, err, "failed to parse alerts")
	})
}

func TestPrometheusCollector_ParseAlerts(t *testing.T) {
	logger := zerolog.Nop()
	collector := NewPrometheusCollector(&fakePrometheusClient{}, &logger)
	start := model.TimeFromUnix(1735689600)

	matrix := model.Matrix{
		{
			Metric: model.Metric{
				"__name__":   "ALERTS",
				"alertstate": "firing",
				"severity":   "warning",
			},
			Values: []model.SamplePair{
				{Timestamp: start, Value: 1},
			},
		},
		{
			Metric: model.Metric{
				"__name__":   "ALERTS",
				"alertname":  "DiskFull",
				"alertstate": "firing",
				"instance":   "node-1",
				"severity":   "critical",
			},
			Values: []model.SamplePair{
				{Timestamp: start, Value: 1},
				{Timestamp: start.Add(60 * time.Second), Value: 0},
			},
		},
	}

	alerts, err := collector.parseAlerts(matrix, "prod", time.Time{}, time.Time{})
	require.NoError(t, err)
	require.Len(t, alerts, 1)

	alert := alerts[0]
	assert.Equal(t, "DiskFull", alert.Name)
	assert.Equal(t, "prod", alert.Cluster)
	assert.Equal(t, "node-1", alert.Labels["instance"])
	assert.Equal(t, "critical", alert.Labels["severity"])
	assert.Equal(t, "inactive", alert.State)
	require.NotNil(t, alert.ResolvedAt)
}

func TestPrometheusCollector_CollectCurrentAlerts(t *testing.T) {
	logger := zerolog.Nop()
	now := model.TimeFromUnix(1735689600)

	t.Run("Success", func(t *testing.T) {
		client := &fakePrometheusClient{
			queryFn: func(_ context.Context, query string, ts time.Time) (model.Value, error) {
				assert.Equal(t, "ALERTS{alertstate=\"firing\"}", query)
				assert.False(t, ts.IsZero())

				return model.Vector{
					&model.Sample{
						Metric: model.Metric{
							"__name__":   "ALERTS",
							"alertname":  "DiskFull",
							"alertstate": "firing",
							"instance":   "node-1",
						},
						Value:     1,
						Timestamp: now,
					},
				}, nil
			},
		}

		collector := NewPrometheusCollector(client, &logger)

		alerts, err := collector.CollectCurrentAlerts(context.Background(), "test-cluster")
		require.NoError(t, err)
		require.Len(t, alerts, 1)
		assert.Equal(t, "DiskFull", alerts[0].Name)
		assert.Equal(t, "test-cluster", alerts[0].Cluster)
		assert.Equal(t, "node-1", alerts[0].Labels["instance"])
		assert.Equal(t, "firing", alerts[0].State)
	})

	t.Run("Invalid Result Type", func(t *testing.T) {
		client := &fakePrometheusClient{
			queryFn: func(_ context.Context, _ string, _ time.Time) (model.Value, error) {
				return model.Matrix{}, nil
			},
		}

		collector := NewPrometheusCollector(client, &logger)

		_, err := collector.CollectCurrentAlerts(context.Background(), "test-cluster")
		require.Error(t, err)
		assert.ErrorContains(t, err, "unexpected result type")
	})
}

func TestCreateAlertKey(t *testing.T) {
	key := createAlertKey("HighCPU", map[string]string{
		"namespace": "prod",
		"instance":  "node-1",
	})

	assert.Equal(t, "HighCPU_instance=node-1_namespace=prod", key)
}
