// Package metrics provides Prometheus metrics collection and a metrics server.
package metrics

import (
	"net/http"
	"time"

	"github.com/neogan/sre-toolkit/internal/alert-analyzer/analyzer"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"
	"github.com/prometheus/client_golang/prometheus/promhttp"
)

var (
	// CommandExecutions tracks CLI command executions
	CommandExecutions = promauto.NewCounterVec(
		prometheus.CounterOpts{
			Name: "sre_toolkit_command_executions_total",
			Help: "Total number of command executions",
		},
		[]string{"command", "status"},
	)

	// CommandDuration tracks command execution duration
	CommandDuration = promauto.NewHistogramVec(
		prometheus.HistogramOpts{
			Name:    "sre_toolkit_command_duration_seconds",
			Help:    "Command execution duration in seconds",
			Buckets: prometheus.DefBuckets,
		},
		[]string{"command"},
	)

	// ResourcesProcessed tracks resources processed by commands
	ResourcesProcessed = promauto.NewCounterVec(
		prometheus.CounterOpts{
			Name: "sre_toolkit_resources_processed_total",
			Help: "Total number of resources processed",
		},
		[]string{"command", "resource_type"},
	)

	// Errors tracks errors by type
	Errors = promauto.NewCounterVec(
		prometheus.CounterOpts{
			Name: "sre_toolkit_errors_total",
			Help: "Total number of errors",
		},
		[]string{"command", "error_type"},
	)

	AlertAnalyzerLastRun = promauto.NewGauge(
		prometheus.GaugeOpts{
			Name: "sre_toolkit_alert_analyzer_last_run_timestamp_seconds",
			Help: "Unix timestamp of the last completed alert-analyzer analysis run",
		},
	)

	AlertAnalyzerSummary = promauto.NewGaugeVec(
		prometheus.GaugeOpts{
			Name: "sre_toolkit_alert_analyzer_summary",
			Help: "Summary metrics from the latest alert-analyzer run",
		},
		[]string{"metric"},
	)

	AlertAnalyzerTopAlertFirings = promauto.NewGaugeVec(
		prometheus.GaugeOpts{
			Name: "sre_toolkit_alert_analyzer_top_alert_firings",
			Help: "Top alert firing counts from the latest alert-analyzer run",
		},
		[]string{"alert_name", "severity"},
	)

	AlertAnalyzerFlappingScore = promauto.NewGaugeVec(
		prometheus.GaugeOpts{
			Name: "sre_toolkit_alert_analyzer_flapping_score",
			Help: "Flapping scores from the latest alert-analyzer run",
		},
		[]string{"alert_name", "severity", "is_flapping"},
	)

	AlertAnalyzerCorrelationScore = promauto.NewGaugeVec(
		prometheus.GaugeOpts{
			Name: "sre_toolkit_alert_analyzer_correlation_score",
			Help: "Correlation scores from the latest alert-analyzer run",
		},
		[]string{"alert_a", "alert_b"},
	)

	AlertAnalyzerTemporalBusinessHoursRatio = promauto.NewGaugeVec(
		prometheus.GaugeOpts{
			Name: "sre_toolkit_alert_analyzer_temporal_business_hours_ratio",
			Help: "Business-hours firing ratio from the latest alert-analyzer run",
		},
		[]string{"alert_name", "severity", "peak_weekday", "peak_hour"},
	)

	AlertAnalyzerRecommendationTotal = promauto.NewGaugeVec(
		prometheus.GaugeOpts{
			Name: "sre_toolkit_alert_analyzer_recommendations_total",
			Help: "Recommendation counts from the latest alert-analyzer run",
		},
		[]string{"category", "priority"},
	)
)

// Config holds metrics server configuration
type Config struct {
	Enabled bool
	Address string
	Path    string
}

// DefaultConfig returns default metrics configuration
func DefaultConfig() *Config {
	return &Config{
		Enabled: false,
		Address: ":9090",
		Path:    "/metrics",
	}
}

// Server represents the metrics HTTP server
type Server struct {
	config *Config
	server *http.Server
}

// NewServer creates a new metrics server
func NewServer(cfg *Config) *Server {
	if cfg == nil {
		cfg = DefaultConfig()
	}

	mux := http.NewServeMux()
	mux.Handle(cfg.Path, promhttp.Handler())

	return &Server{
		config: cfg,
		server: &http.Server{
			Addr:              cfg.Address,
			Handler:           mux,
			ReadHeaderTimeout: 10 * time.Second,
		},
	}
}

// Start starts the metrics server
func (s *Server) Start() error {
	if !s.config.Enabled {
		return nil
	}
	return s.server.ListenAndServe()
}

// Stop stops the metrics server
func (s *Server) Stop() error {
	if !s.config.Enabled {
		return nil
	}
	return s.server.Close()
}

// SetAlertAnalyzerMetrics updates the latest alert-analyzer analysis gauges.
func SetAlertAnalyzerMetrics(
	stats analyzer.SummaryStats,
	frequency []analyzer.FrequencyResult,
	flapping []analyzer.FlappingResult,
	correlation []analyzer.CorrelationResult,
	temporal []analyzer.TemporalResult,
	recommendations []analyzer.Recommendation,
) {
	AlertAnalyzerLastRun.SetToCurrentTime()

	AlertAnalyzerSummary.WithLabelValues("total_alerts").Set(float64(stats.TotalAlerts))
	AlertAnalyzerSummary.WithLabelValues("unique_alerts").Set(float64(stats.UniqueAlerts))
	AlertAnalyzerSummary.WithLabelValues("total_firings").Set(float64(stats.TotalFirings))
	AlertAnalyzerSummary.WithLabelValues("avg_duration_seconds").Set(stats.AvgDuration.Seconds())
	AlertAnalyzerSummary.WithLabelValues("total_firing_time_seconds").Set(stats.TotalFiringTime.Seconds())

	AlertAnalyzerTopAlertFirings.Reset()
	for _, result := range frequency {
		AlertAnalyzerTopAlertFirings.WithLabelValues(result.AlertName, result.Severity).Set(float64(result.FiringCount))
	}

	AlertAnalyzerFlappingScore.Reset()
	for _, result := range flapping {
		isFlapping := "false"
		if result.IsFlapping {
			isFlapping = "true"
		}
		AlertAnalyzerFlappingScore.WithLabelValues(result.AlertName, result.Severity, isFlapping).Set(result.FlappingScore)
	}

	AlertAnalyzerCorrelationScore.Reset()
	for _, result := range correlation {
		AlertAnalyzerCorrelationScore.WithLabelValues(result.AlertA, result.AlertB).Set(result.CorrelationScore)
	}

	AlertAnalyzerTemporalBusinessHoursRatio.Reset()
	for _, result := range temporal {
		AlertAnalyzerTemporalBusinessHoursRatio.WithLabelValues(
			result.AlertName,
			result.Severity,
			result.PeakWeekday,
			formatMetricHour(result.PeakHour),
		).Set(result.BusinessHoursRatio)
	}

	AlertAnalyzerRecommendationTotal.Reset()
	seen := make(map[string]float64)
	for _, result := range recommendations {
		key := result.Category + "\x00" + result.Priority
		seen[key]++
	}
	for key, count := range seen {
		parts := splitMetricKey(key)
		AlertAnalyzerRecommendationTotal.WithLabelValues(parts[0], parts[1]).Set(count)
	}
}

func formatMetricHour(hour int) string {
	return time.Date(2000, 1, 1, hour, 0, 0, 0, time.UTC).Format("15:04")
}

func splitMetricKey(key string) []string {
	for i := 0; i < len(key); i++ {
		if key[i] == 0 {
			return []string{key[:i], key[i+1:]}
		}
	}
	return []string{key, ""}
}
