package main

import (
	"context"
	"fmt"
	"os"
	"time"

	"github.com/spf13/cobra"

	"github.com/neogan/sre-toolkit/internal/alert-analyzer/analyzer"
	"github.com/neogan/sre-toolkit/internal/alert-analyzer/collector"
	"github.com/neogan/sre-toolkit/internal/alert-analyzer/reporter"
	"github.com/neogan/sre-toolkit/internal/alert-analyzer/storage"
	"github.com/neogan/sre-toolkit/pkg/cli"
	"github.com/neogan/sre-toolkit/pkg/config"
	"github.com/neogan/sre-toolkit/pkg/logging"
	"github.com/neogan/sre-toolkit/pkg/metrics"
	"github.com/neogan/sre-toolkit/pkg/prometheus"
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
	rootCmd.AddCommand(newVersionCmd())

	// Execute
	if err := rootCmd.Execute(); err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}
}

func newAnalyzeCmd() *cobra.Command {
	var (
		prometheusURL string
		lookback      string
		resolution    string
		output        string
		topN          int
		timeout       string
		insecure      bool
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
  alert-analyzer analyze --prometheus-url http://prom:9090 --top-n 20 --output json`,
		RunE: func(cmd *cobra.Command, args []string) error {
			return runAnalyze(prometheusURL, lookback, resolution, output, topN, timeout, insecure)
		},
	}

	// Add flags
	cmd.Flags().StringVar(&prometheusURL, "prometheus-url", "", "Prometheus server URL (required)")
	cmd.Flags().StringVar(&lookback, "lookback", "7d", "Time range to analyze (e.g., 7d, 24h, 30d)")
	cmd.Flags().StringVar(&resolution, "resolution", "5m", "Query resolution (e.g., 1m, 5m, 15m)")
	cmd.Flags().StringVarP(&output, "output", "o", "table", "Output format: table or json")
	cmd.Flags().IntVar(&topN, "top-n", 20, "Number of top alerts to show")
	cmd.Flags().StringVar(&timeout, "timeout", "30s", "Request timeout")
	cmd.Flags().BoolVar(&insecure, "insecure", false, "Skip TLS verification")

	cmd.MarkFlagRequired("prometheus-url")

	return cmd
}

func newVersionCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "version",
		Short: "Show version information",
		Run: func(cmd *cobra.Command, args []string) {
			fmt.Printf("alert-analyzer version %s\n", version)
		},
	}
}

func runAnalyze(prometheusURL, lookbackStr, resolutionStr, outputFormat string, topN int, timeoutStr string, insecure bool) error {
	// Initialize configuration
	cfg := config.Default()

	// Initialize logging
	logging.Init(cfg.Logging)
	logger := logging.GetLogger()
	logger.Info().Msg("Starting alert-analyzer")

	// Start metrics server if enabled
	if cfg.Metrics.Enabled {
		metricsServer := metrics.NewServer(cfg.Metrics)
		go func() {
			if err := metricsServer.Start(); err != nil {
				logger.Error().Err(err).Msg("Failed to start metrics server")
			}
		}()
	}

	// Parse duration parameters
	lookback, err := time.ParseDuration(lookbackStr)
	if err != nil {
		return fmt.Errorf("invalid lookback duration: %w", err)
	}

	resolution, err := time.ParseDuration(resolutionStr)
	if err != nil {
		return fmt.Errorf("invalid resolution duration: %w", err)
	}

	timeout, err := time.ParseDuration(timeoutStr)
	if err != nil {
		return fmt.Errorf("invalid timeout duration: %w", err)
	}

	// Create Prometheus client
	promClient, err := prometheus.NewClient(&prometheus.Config{
		URL:      prometheusURL,
		Timeout:  timeout,
		Insecure: insecure,
	}, &logger)
	if err != nil {
		return fmt.Errorf("failed to create Prometheus client: %w", err)
	}

	// Test connectivity
	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()

	if err := promClient.Ping(ctx); err != nil {
		logger.Error().Err(err).Msg("Failed to connect to Prometheus")
		return fmt.Errorf("failed to connect to Prometheus: %w", err)
	}

	logger.Info().Str("url", prometheusURL).Msg("Connected to Prometheus")

	// Create collector
	promCollector := collector.NewPrometheusCollector(promClient, &logger)

	// Create storage
	store := storage.NewMemoryStorage()

	// Collect alert data
	logger.Info().
		Dur("lookback", lookback).
		Dur("resolution", resolution).
		Msg("Collecting alert data")

	ctx, cancel = context.WithTimeout(context.Background(), timeout)
	defer cancel()

	history, err := promCollector.Collect(ctx, lookback, resolution)
	if err != nil {
		logger.Error().Err(err).Msg("Failed to collect alert data")
		return fmt.Errorf("failed to collect alert data: %w", err)
	}

	// Store alert history
	if err := store.Store(history); err != nil {
		return fmt.Errorf("failed to store alert history: %w", err)
	}

	logger.Info().
		Int("total_alerts", history.CountAlerts()).
		Int("unique_alerts", history.CountUniqueAlerts()).
		Msg("Alert data collected")

	// Analyze frequency
	frequencyAnalyzer := analyzer.NewFrequencyAnalyzer(history)

	// Get summary stats
	stats := frequencyAnalyzer.GetSummaryStats()

	// Get top N alerts
	topAlerts := frequencyAnalyzer.AnalyzeTopN(topN)

	logger.Info().
		Int("total_firings", stats.TotalFirings).
		Int("unique_alerts", stats.UniqueAlerts).
		Msg("Analysis complete")

	// Report results
	rep := reporter.NewReporter(outputFormat, os.Stdout)

	if err := rep.ReportComplete(stats, topAlerts); err != nil {
		return fmt.Errorf("failed to generate report: %w", err)
	}

	// Record metrics
	if cfg.Metrics.Enabled {
		metrics.CommandExecutions.WithLabelValues("analyze", "success").Inc()
		metrics.ResourcesProcessed.WithLabelValues("analyze", "alerts").Add(float64(history.CountAlerts()))
	}

	logger.Info().Msg("Analysis complete")
	return nil
}
