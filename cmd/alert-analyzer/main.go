// Package main provides the entry point for the alert-analyzer tool.
package main

import (
	"context"
	"fmt"
	"net/url"
	"os"
	"os/signal"
	"strings"
	"syscall"
	"time"

	"github.com/spf13/cobra"

	"github.com/neogan/sre-toolkit/internal/alert-analyzer/collector"
	"github.com/neogan/sre-toolkit/pkg/alertmanager"
	"github.com/neogan/sre-toolkit/pkg/cli"
	"github.com/neogan/sre-toolkit/pkg/config"
	"github.com/neogan/sre-toolkit/pkg/logging"
	"github.com/neogan/sre-toolkit/pkg/metrics"
	"github.com/neogan/sre-toolkit/pkg/tracing"
	"github.com/rs/zerolog"
)

var (
	version = "0.1.0"
)

func main() {
	// Create root command using shared CLI framework
	rootCmd := cli.NewRootCmd()
	rootCmd.Use = "alert-analyzer"
	rootCmd.Short = "Alert analysis and optimization tool"
	rootCmd.Long = `alert-analyzer analyzes Prometheus alert history to identify noisy alerts,
flapping patterns, and correlations. It provides actionable recommendations
to reduce alert fatigue and improve alerting effectiveness.`
	rootCmd.Version = version

	// Add subcommands
	rootCmd.AddCommand(newAnalyzeCmd())
	rootCmd.AddCommand(newMonitorCmd())
	rootCmd.AddCommand(newVersionCmd())

	// Execute
	if err := rootCmd.Execute(); err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}
}

func newAnalyzeCmd() *cobra.Command {
	var (
		prometheusURLs       []string
		alertmanagerURL      string
		lookback             string
		resolution           string
		output               string
		topN                 int
		timeout              string
		insecure             bool
		showFlapping         bool
		showCorrelation      bool
		showTemporalPatterns bool
		showRecommendations  bool
		flappingThreshold    float64
	)

	cmd := &cobra.Command{
		Use:   "analyze",
		Short: "Analyze alert history from Prometheus",
		Long: `Analyze alert history from Prometheus to identify patterns and generate recommendations.

This command connects to a Prometheus server, queries alert history over a specified
time range, and performs frequency analysis to identify the most problematic alerts.`,
		Example: `  # Analyze last 7 days with default settings
  alert-analyzer analyze --prometheus-url http://localhost:9090

  # Analyze last 30 days with custom resolution
  alert-analyzer analyze --prometheus-url http://prom:9090 --lookback 30d --resolution 15m

  # Show top 20 alerts in JSON format
  alert-analyzer analyze --prometheus-url http://prom:9090 --top-n 20 --output json

  # Include flapping analysis with custom threshold
  alert-analyzer analyze --prometheus-url http://prom:9090 --show-flapping --flapping-threshold 5.0

  # Include alert correlation analysis
  alert-analyzer analyze --prometheus-url http://prom:9090 --show-correlation

  # Show temporal alert patterns
  alert-analyzer analyze --prometheus-url http://prom:9090 --show-temporal-patterns

  # Generate actionable recommendations
  alert-analyzer analyze --prometheus-url http://prom:9090 --show-recommendations`,
		RunE: func(cmd *cobra.Command, args []string) error {
			return runAnalyze(analysisOptions{
				prometheusURLs:       prometheusURLs,
				alertmanagerURL:      alertmanagerURL,
				lookbackStr:          lookback,
				resolutionStr:        resolution,
				outputFormat:         output,
				topN:                 topN,
				timeoutStr:           timeout,
				insecure:             insecure,
				showFlapping:         showFlapping,
				showCorrelation:      showCorrelation,
				showTemporalPatterns: showTemporalPatterns,
				showRecommendations:  showRecommendations,
				flappingThreshold:    flappingThreshold,
			})
		},
	}

	// Add flags
	cmd.Flags().StringSliceVar(&prometheusURLs, "prometheus-url", nil, "Prometheus server URL(s) in format [cluster=]url (required)")
	cmd.Flags().StringVar(&alertmanagerURL, "alertmanager-url", "", "Alertmanager server URL (optional)")
	cmd.Flags().StringVar(&lookback, "lookback", "7d", "Time range to analyze (e.g., 7d, 24h, 30d)")
	cmd.Flags().StringVar(&resolution, "resolution", "5m", "Query resolution (e.g., 1m, 5m, 15m)")
	cmd.Flags().StringVarP(&output, "output", "o", "table", "Output format: table, json, or markdown")
	cmd.Flags().IntVar(&topN, "top-n", 20, "Number of top alerts to show")
	cmd.Flags().StringVar(&timeout, "timeout", "30s", "Request timeout")
	cmd.Flags().BoolVar(&insecure, "insecure", false, "Skip TLS verification")
	cmd.Flags().BoolVar(&showFlapping, "show-flapping", false, "Include flapping alerts analysis")
	cmd.Flags().BoolVar(&showCorrelation, "show-correlation", false, "Include alert correlation analysis")
	cmd.Flags().BoolVar(&showTemporalPatterns, "show-temporal-patterns", false, "Include time-of-day and day-of-week alert patterns")
	cmd.Flags().BoolVar(&showRecommendations, "show-recommendations", false, "Include actionable recommendations based on alert patterns")
	cmd.Flags().Float64Var(&flappingThreshold, "flapping-threshold", 3.0, "Flapping threshold (transitions per hour)")

	cmd.MarkFlagRequired("prometheus-url")

	return cmd
}

func newMonitorCmd() *cobra.Command {
	var (
		prometheusURLs       []string
		alertmanagerURL      string
		lookback             string
		resolution           string
		topN                 int
		timeout              string
		insecure             bool
		showFlapping         bool
		showCorrelation      bool
		showTemporalPatterns bool
		showRecommendations  bool
		flappingThreshold    float64
		interval             string
		metricsAddress       string
		metricsPath          string
	)

	cmd := &cobra.Command{
		Use:   "monitor",
		Short: "Continuously analyze alerts and expose Prometheus metrics",
		Long: `Run alert analysis on a fixed interval and keep exporting analysis metrics
for Prometheus and Grafana dashboards.`,
		Example: `  # Export alert analysis metrics every minute
  alert-analyzer monitor \
    --prometheus-url http://prom:9090 \
    --show-flapping \
    --show-correlation \
    --show-temporal-patterns \
    --show-recommendations \
    --interval 1m \
    --metrics-address :8080`,
		RunE: func(cmd *cobra.Command, args []string) error {
			return runMonitor(analysisOptions{
				prometheusURLs:       prometheusURLs,
				alertmanagerURL:      alertmanagerURL,
				lookbackStr:          lookback,
				resolutionStr:        resolution,
				topN:                 topN,
				timeoutStr:           timeout,
				insecure:             insecure,
				showFlapping:         showFlapping,
				showCorrelation:      showCorrelation,
				showTemporalPatterns: showTemporalPatterns,
				showRecommendations:  showRecommendations,
				flappingThreshold:    flappingThreshold,
			}, interval, metricsAddress, metricsPath)
		},
	}

	cmd.Flags().StringSliceVar(&prometheusURLs, "prometheus-url", nil, "Prometheus server URL(s) in format [cluster=]url (required)")
	cmd.Flags().StringVar(&alertmanagerURL, "alertmanager-url", "", "Alertmanager server URL (optional)")
	cmd.Flags().StringVar(&lookback, "lookback", "7d", "Time range to analyze (e.g., 7d, 24h, 30d)")
	cmd.Flags().StringVar(&resolution, "resolution", "5m", "Query resolution (e.g., 1m, 5m, 15m)")
	cmd.Flags().IntVar(&topN, "top-n", 20, "Number of top alerts to export as metrics")
	cmd.Flags().StringVar(&timeout, "timeout", "30s", "Request timeout")
	cmd.Flags().BoolVar(&insecure, "insecure", false, "Skip TLS verification")
	cmd.Flags().BoolVar(&showFlapping, "show-flapping", true, "Include flapping alerts analysis")
	cmd.Flags().BoolVar(&showCorrelation, "show-correlation", true, "Include alert correlation analysis")
	cmd.Flags().BoolVar(&showTemporalPatterns, "show-temporal-patterns", true, "Include time-of-day and day-of-week alert patterns")
	cmd.Flags().BoolVar(&showRecommendations, "show-recommendations", true, "Include actionable recommendations based on alert patterns")
	cmd.Flags().Float64Var(&flappingThreshold, "flapping-threshold", 3.0, "Flapping threshold (transitions per hour)")
	cmd.Flags().StringVar(&interval, "interval", "1m", "Analysis refresh interval")
	cmd.Flags().StringVar(&metricsAddress, "metrics-address", ":8080", "Metrics listen address")
	cmd.Flags().StringVar(&metricsPath, "metrics-path", "/metrics", "Metrics HTTP path")
	cmd.MarkFlagRequired("prometheus-url")

	return cmd
}

func newVersionCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "version",
		Short: "Show version information",
		Run: func(cmd *cobra.Command, args []string) {
			fmt.Fprintf(cmd.OutOrStdout(), "alert-analyzer version %s\n", version)
		},
	}
}

// parsePrometheusURL parses a [cluster=]url string
func parsePrometheusURL(input string) (clusterName, targetURL string) {
	parts := strings.SplitN(input, "=", 2)
	if len(parts) == 2 {
		return parts[0], parts[1]
	}

	// If no cluster name provided, derive from hostname
	u, err := url.Parse(input)
	if err == nil && u.Host != "" {
		return u.Host, input
	}

	return "default", input
}

func runAnalyze(opts analysisOptions) error {
	// Initialize configuration
	cfg := config.Default()

	// Initialize logging
	logging.Init(cfg.Logging)
	logger := logging.GetLogger()
	logger.Info().Msg("Starting alert-analyzer")

	// Initialize tracing
	shutdownTracer, err := tracing.InitTracer("alert-analyzer", *cfg.Tracing)
	if err != nil {
		logger.Error().Err(err).Msg("Failed to initialize tracing")
	} else {
		defer shutdownTracer(context.Background())
	}

	// Start metrics server if enabled
	if cfg.Metrics.Enabled {
		metricsServer := metrics.NewServer(cfg.Metrics)
		go func() {
			if err := metricsServer.Start(); err != nil {
				logger.Error().Err(err).Msg("Failed to start metrics server")
			}
		}()
	}

	result, err := performAnalysis(opts, logger)
	if err != nil {
		return err
	}

	if err := reportAnalysis(result, opts.outputFormat); err != nil {
		return fmt.Errorf("failed to generate report: %w", err)
	}

	if cfg.Metrics.Enabled {
		recordAnalysisMetrics(result, "analyze")
	}

	logger.Info().Msg("Analysis complete")
	return nil
}

func runMonitor(opts analysisOptions, intervalStr, metricsAddress, metricsPath string) error {
	interval, err := time.ParseDuration(intervalStr)
	if err != nil {
		return fmt.Errorf("invalid interval duration: %w", err)
	}

	cfg := config.Default()
	logging.Init(cfg.Logging)
	logger := logging.GetLogger()
	logger.Info().
		Str("metrics_address", metricsAddress).
		Str("metrics_path", metricsPath).
		Dur("interval", interval).
		Msg("Starting alert-analyzer monitor")

	metricsServer := metrics.NewServer(&metrics.Config{
		Enabled: true,
		Address: metricsAddress,
		Path:    metricsPath,
	})
	go func() {
		if err := metricsServer.Start(); err != nil {
			logger.Error().Err(err).Msg("Failed to start metrics server")
		}
	}()

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	ticker := time.NewTicker(interval)
	defer ticker.Stop()

	for {
		start := time.Now()
		result, err := performAnalysis(opts, logger)
		if err != nil {
			logger.Error().Err(err).Msg("Monitor analysis cycle failed")
			metrics.Errors.WithLabelValues("monitor", "analysis").Inc()
		} else {
			recordAnalysisMetrics(result, "monitor")
			metrics.CommandDuration.WithLabelValues("monitor").Observe(time.Since(start).Seconds())
			logger.Info().
				Int("alerts", result.history.CountAlerts()).
				Int("recommendations", len(result.recommendations)).
				Msg("Monitor analysis cycle complete")
		}

		select {
		case <-ctx.Done():
			logger.Info().Msg("Stopping alert-analyzer monitor")
			return nil
		case <-ticker.C:
		}
	}
}

func collectAlertmanagerData(ctx context.Context, url string, timeout time.Duration, insecure bool, logger zerolog.Logger) error {
	amClient, err := alertmanager.NewClient(&alertmanager.Config{
		URL:      url,
		Timeout:  timeout,
		Insecure: insecure,
	}, &logger)
	if err != nil {
		return fmt.Errorf("failed to create Alertmanager client: %w", err)
	}

	if err := amClient.Ping(ctx); err != nil {
		return fmt.Errorf("failed to connect to Alertmanager: %w", err)
	}

	amCollector := collector.NewAlertmanagerCollector(amClient, &logger)
	amHistory, err := amCollector.CollectCurrentAlerts(ctx)
	if err != nil {
		return fmt.Errorf("failed to collect from Alertmanager: %w", err)
	}

	logger.Info().Int("active_alerts", amHistory.CountAlerts()).Msg("Collected active alerts from Alertmanager")
	return nil
}
