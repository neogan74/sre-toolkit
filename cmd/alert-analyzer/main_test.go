package main

import (
	"bytes"
	"testing"
	"time"

	"github.com/rs/zerolog"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestParsePrometheusURL(t *testing.T) {
	tests := []struct {
		name          string
		input         string
		wantCluster   string
		wantTargetURL string
	}{
		{
			name:          "Explicit Cluster",
			input:         "prod=http://prometheus:9090",
			wantCluster:   "prod",
			wantTargetURL: "http://prometheus:9090",
		},
		{
			name:          "Derived From Host",
			input:         "https://prometheus.example.com:9090",
			wantCluster:   "prometheus.example.com:9090",
			wantTargetURL: "https://prometheus.example.com:9090",
		},
		{
			name:          "Fallback To Default",
			input:         "not-a-url",
			wantCluster:   "default",
			wantTargetURL: "not-a-url",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cluster, targetURL := parsePrometheusURL(tt.input)
			assert.Equal(t, tt.wantCluster, cluster)
			assert.Equal(t, tt.wantTargetURL, targetURL)
		})
	}
}

func TestNewAnalyzeCmd_RequiresPrometheusURL(t *testing.T) {
	cmd := newAnalyzeCmd()
	cmd.SetArgs(nil)
	cmd.SetOut(new(bytes.Buffer))
	cmd.SetErr(new(bytes.Buffer))

	err := cmd.Execute()
	require.Error(t, err)
	assert.ErrorContains(t, err, `required flag(s) "prometheus-url" not set`)
}

func TestNewVersionCmd(t *testing.T) {
	cmd := newVersionCmd()
	var stdout bytes.Buffer
	cmd.SetOut(&stdout)

	cmd.Run(cmd, nil)

	assert.Equal(t, "alert-analyzer version "+version+"\n", stdout.String())
}

func TestRunAnalyzeValidationErrors(t *testing.T) {
	tests := []struct {
		name           string
		prometheusURLs []string
		lookback       string
		resolution     string
		timeout        string
		wantErr        string
	}{
		{
			name:           "Missing Prometheus URL",
			prometheusURLs: nil,
			lookback:       "7d",
			resolution:     "5m",
			timeout:        "30s",
			wantErr:        "at least one prometheus-url is required",
		},
		{
			name:           "Invalid Lookback",
			prometheusURLs: []string{"http://prometheus:9090"},
			lookback:       "not-a-duration",
			resolution:     "5m",
			timeout:        "30s",
			wantErr:        "invalid lookback duration",
		},
		{
			name:           "Invalid Resolution",
			prometheusURLs: []string{"http://prometheus:9090"},
			lookback:       "168h",
			resolution:     "bad",
			timeout:        "30s",
			wantErr:        "invalid resolution duration",
		},
		{
			name:           "Invalid Timeout",
			prometheusURLs: []string{"http://prometheus:9090"},
			lookback:       "168h",
			resolution:     "5m",
			timeout:        "bad",
			wantErr:        "invalid timeout duration",
		},
		{
			name:           "Invalid Prometheus Client URL",
			prometheusURLs: []string{"://bad"},
			lookback:       "168h",
			resolution:     "5m",
			timeout:        "30s",
			wantErr:        "failed to collect alert data from any of the provided Prometheus sources",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := runAnalyze(analysisOptions{
				prometheusURLs:       tt.prometheusURLs,
				alertmanagerURL:      "",
				lookbackStr:          tt.lookback,
				resolutionStr:        tt.resolution,
				outputFormat:         "table",
				topN:                 20,
				timeoutStr:           tt.timeout,
				insecure:             false,
				showFlapping:         false,
				showCorrelation:      false,
				showTemporalPatterns: false,
				showRecommendations:  false,
				flappingThreshold:    3.0,
			})
			require.Error(t, err)
			assert.ErrorContains(t, err, tt.wantErr)
		})
	}
}

func TestCollectAlertmanagerData_InvalidURL(t *testing.T) {
	err := collectAlertmanagerData(t.Context(), "://bad", 30*time.Second, false, zerolog.Nop())
	require.Error(t, err)
	assert.ErrorContains(t, err, "failed to create Alertmanager client")
}
