package collector

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/rs/zerolog"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	ampkg "github.com/neogan/sre-toolkit/pkg/alertmanager"
)

type fakeAlertmanagerClient struct {
	listAlertsFn func(ctx context.Context, filter []string) ([]ampkg.Alert, error)
}

func (f *fakeAlertmanagerClient) ListAlerts(ctx context.Context, filter []string) ([]ampkg.Alert, error) {
	if f.listAlertsFn == nil {
		return nil, errors.New("unexpected ListAlerts call")
	}
	return f.listAlertsFn(ctx, filter)
}

func TestAlertmanagerCollector_CollectCurrentAlerts(t *testing.T) {
	logger := zerolog.Nop()
	now := time.Now().UTC()

	t.Run("Success", func(t *testing.T) {
		client := &fakeAlertmanagerClient{
			listAlertsFn: func(_ context.Context, filter []string) ([]ampkg.Alert, error) {
				assert.Nil(t, filter)
				return []ampkg.Alert{
					{
						Labels: map[string]string{
							"alertname": "HighCPU",
							"severity":  "critical",
						},
						Annotations: map[string]string{
							"summary": "CPU is high",
						},
						StartsAt: now.Add(-5 * time.Minute),
					},
					{
						Labels: map[string]string{
							"severity": "warning",
						},
						EndsAt:   now.Add(-time.Minute),
						StartsAt: now.Add(-10 * time.Minute),
					},
				}, nil
			},
		}

		collector := NewAlertmanagerCollector(client, &logger)

		history, err := collector.CollectCurrentAlerts(context.Background())
		require.NoError(t, err)
		require.Len(t, history.Alerts, 2)
		assert.Equal(t, "alertmanager", history.Source)

		assert.Equal(t, "HighCPU", history.Alerts[0].Name)
		assert.Equal(t, "firing", history.Alerts[0].State)
		assert.Equal(t, "CPU is high", history.Alerts[0].Annotations["summary"])

		assert.Equal(t, "unknown", history.Alerts[1].Name)
		assert.Equal(t, "resolved", history.Alerts[1].State)
		require.NotNil(t, history.Alerts[1].ResolvedAt)
	})

	t.Run("API Error", func(t *testing.T) {
		client := &fakeAlertmanagerClient{
			listAlertsFn: func(_ context.Context, _ []string) ([]ampkg.Alert, error) {
				return nil, errors.New("boom")
			},
		}

		collector := NewAlertmanagerCollector(client, &logger)

		_, err := collector.CollectCurrentAlerts(context.Background())
		require.Error(t, err)
		assert.ErrorContains(t, err, "failed to list alerts")
	})
}
