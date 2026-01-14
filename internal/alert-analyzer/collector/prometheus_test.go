package collector

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/neogan/sre-toolkit/pkg/prometheus"
	"github.com/rs/zerolog"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestPrometheusCollector_Collect(t *testing.T) {
	logger := zerolog.Nop()

	t.Run("Success", func(t *testing.T) {
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			assert.Contains(t, r.URL.Path, "/api/v1/query_range")
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusOK)
			// Mock response with one alert stream
			response := `{
				"status": "success",
				"data": {
					"resultType": "matrix",
					"result": [
						{
							"metric": {
								"__name__": "ALERTS",
								"alertname": "HighCPU",
								"alertstate": "firing",
								"severity": "critical"
							},
							"values": [
								[1735689600, "1"],
								[1735689660, "1"],
								[1735689720, "0"]
							]
						}
					]
				}
			}`
			w.Write([]byte(response))
		}))
		defer server.Close()

		client, err := prometheus.NewClient(&prometheus.Config{URL: server.URL}, &logger)
		require.NoError(t, err)

		collector := NewPrometheusCollector(client, &logger)

		history, err := collector.Collect(context.Background(), 1*time.Hour, 1*time.Minute)
		assert.NoError(t, err)
		assert.NotNil(t, history)
		assert.Equal(t, 1, history.CountUniqueAlerts())

		alerts := history.Alerts
		require.NotEmpty(t, alerts)
		assert.Equal(t, "HighCPU", alerts[0].Name)
		assert.Equal(t, "critical", alerts[0].Labels["severity"])

		// The alert became 0 at the last sample, so it should be resolved
		assert.Equal(t, "inactive", alerts[0].State)
		assert.True(t, alerts[0].IsResolved())
	})

	t.Run("Empty Result", func(t *testing.T) {
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusOK)
			w.Write([]byte(`{"status":"success","data":{"resultType":"matrix","result":[]}}`))
		}))
		defer server.Close()

		client, err := prometheus.NewClient(&prometheus.Config{URL: server.URL}, &logger)
		require.NoError(t, err)

		collector := NewPrometheusCollector(client, &logger)

		history, err := collector.Collect(context.Background(), 1*time.Hour, 1*time.Minute)
		assert.NoError(t, err)
		assert.Equal(t, 0, history.CountAlerts())
	})

	t.Run("API Error", func(t *testing.T) {
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusInternalServerError)
		}))
		defer server.Close()

		client, err := prometheus.NewClient(&prometheus.Config{URL: server.URL}, &logger)
		require.NoError(t, err)

		collector := NewPrometheusCollector(client, &logger)

		_, err = collector.Collect(context.Background(), 1*time.Hour, 1*time.Minute)
		assert.Error(t, err)
	})
}

func TestPrometheusCollector_CollectCurrentAlerts(t *testing.T) {
	logger := zerolog.Nop()

	t.Run("Success", func(t *testing.T) {
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			assert.Contains(t, r.URL.Path, "/api/v1/query")

			// Check query parameter from URL (GET) or Body (POST)
			q := r.URL.Query().Get("query")
			if q == "" {
				r.ParseForm()
				q = r.Form.Get("query")
			}
			assert.True(t, strings.Contains(q, "ALERTS"))

			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusOK)
			response := `{
				"status": "success",
				"data": {
					"resultType": "vector",
					"result": [
						{
							"metric": {
								"__name__": "ALERTS",
								"alertname": "DiskFull",
								"alertstate": "firing",
								"instance": "node-1"
							},
							"value": [1735689600, "1"]
						}
					]
				}
			}`
			w.Write([]byte(response))
		}))
		defer server.Close()

		client, err := prometheus.NewClient(&prometheus.Config{URL: server.URL}, &logger)
		require.NoError(t, err)

		collector := NewPrometheusCollector(client, &logger)

		alerts, err := collector.CollectCurrentAlerts(context.Background())
		assert.NoError(t, err)
		assert.Len(t, alerts, 1)
		assert.Equal(t, "DiskFull", alerts[0].Name)
		assert.Equal(t, "node-1", alerts[0].Labels["instance"])
	})
}
