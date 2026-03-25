package main

import (
	"context"
	"fmt"
	"os"
	"time"

	"github.com/rs/zerolog"

	"github.com/neogan/sre-toolkit/internal/alert-analyzer/analyzer"
	"github.com/neogan/sre-toolkit/internal/alert-analyzer/collector"
	"github.com/neogan/sre-toolkit/internal/alert-analyzer/reporter"
	"github.com/neogan/sre-toolkit/internal/alert-analyzer/storage"
	toolkitmetrics "github.com/neogan/sre-toolkit/pkg/metrics"
	"github.com/neogan/sre-toolkit/pkg/prometheus"
)

type analysisOptions struct {
	prometheusURLs       []string
	alertmanagerURL      string
	lookbackStr          string
	resolutionStr        string
	outputFormat         string
	topN                 int
	timeoutStr           string
	insecure             bool
	showFlapping         bool
	showCorrelation      bool
	showTemporalPatterns bool
	showRecommendations  bool
	flappingThreshold    float64
}

type analysisResult struct {
	stats           analyzer.SummaryStats
	topAlerts       []analyzer.FrequencyResult
	allFrequency    []analyzer.FrequencyResult
	flapping        []analyzer.FlappingResult
	correlation     []analyzer.CorrelationResult
	temporal        []analyzer.TemporalResult
	recommendations []analyzer.Recommendation
	history         *collector.AlertHistory
}

func performAnalysis(opts analysisOptions, logger zerolog.Logger) (*analysisResult, error) {
	if len(opts.prometheusURLs) == 0 {
		return nil, fmt.Errorf("at least one prometheus-url is required")
	}

	lookback, err := time.ParseDuration(opts.lookbackStr)
	if err != nil {
		return nil, fmt.Errorf("invalid lookback duration: %w", err)
	}

	resolution, err := time.ParseDuration(opts.resolutionStr)
	if err != nil {
		return nil, fmt.Errorf("invalid resolution duration: %w", err)
	}

	timeout, err := time.ParseDuration(opts.timeoutStr)
	if err != nil {
		return nil, fmt.Errorf("invalid timeout duration: %w", err)
	}

	store := storage.NewMemoryStorage()

	logger.Info().
		Dur("lookback", lookback).
		Dur("resolution", resolution).
		Msg("Collecting alert data")

	var aggregatedHistory *collector.AlertHistory
	allRules := make([]collector.AlertRule, 0)

	for _, urlInput := range opts.prometheusURLs {
		clusterName, promURL := parsePrometheusURL(urlInput)
		logger.Info().Str("cluster", clusterName).Str("url", promURL).Msg("Connecting to Prometheus")

		promClient, err := prometheus.NewClient(&prometheus.Config{
			URL:      promURL,
			Timeout:  timeout,
			Insecure: opts.insecure,
		}, &logger)
		if err != nil {
			logger.Error().Err(err).Str("cluster", clusterName).Msg("Failed to create Prometheus client")
			continue
		}

		ctx, cancel := context.WithTimeout(context.Background(), timeout)
		if err := promClient.Ping(ctx); err != nil {
			logger.Error().Err(err).Str("cluster", clusterName).Msg("Failed to connect to Prometheus")
			cancel()
			continue
		}

		if opts.showRecommendations {
			ruleCollector := collector.NewRuleCollector(promClient, &logger)
			rules, rulesErr := ruleCollector.CollectAlertRules(ctx, clusterName)
			if rulesErr != nil {
				logger.Error().Err(rulesErr).Str("cluster", clusterName).Msg("Failed to collect alert rules")
			} else {
				allRules = append(allRules, rules...)
			}
		}

		promCollector := collector.NewPrometheusCollector(promClient, &logger)
		history, err := promCollector.Collect(ctx, clusterName, lookback, resolution)
		cancel()
		if err != nil {
			logger.Error().Err(err).Str("cluster", clusterName).Msg("Failed to collect alert data")
			continue
		}

		if aggregatedHistory == nil {
			aggregatedHistory = history
		} else {
			aggregatedHistory.Merge(history)
		}
	}

	if aggregatedHistory == nil && opts.showRecommendations && len(allRules) > 0 {
		aggregatedHistory = &collector.AlertHistory{
			StartTime: time.Now().Add(-lookback),
			EndTime:   time.Now(),
			Source:    "prometheus",
		}
	}

	if aggregatedHistory == nil || (aggregatedHistory.CountAlerts() == 0 && !(opts.showRecommendations && len(allRules) > 0)) {
		return nil, fmt.Errorf("failed to collect alert data from any of the provided Prometheus sources")
	}

	if err := store.Store(aggregatedHistory); err != nil {
		return nil, fmt.Errorf("failed to store alert history: %w", err)
	}

	logger.Info().
		Int("total_alerts", aggregatedHistory.CountAlerts()).
		Int("unique_alerts", aggregatedHistory.CountUniqueAlerts()).
		Msg("Alert data collected")

	frequencyAnalyzer := analyzer.NewFrequencyAnalyzer(aggregatedHistory)
	stats := frequencyAnalyzer.GetSummaryStats()
	allFrequency := frequencyAnalyzer.Analyze()
	topAlerts := limitFrequencyResults(allFrequency, opts.topN)

	logger.Info().
		Int("total_firings", stats.TotalFirings).
		Int("unique_alerts", stats.UniqueAlerts).
		Msg("Frequency analysis complete")

	allCorrelations := []analyzer.CorrelationResult{}
	correlations := []analyzer.CorrelationResult{}
	if opts.showCorrelation || opts.showRecommendations {
		correlationAnalyzer := analyzer.NewCorrelationAnalyzer(aggregatedHistory)
		allCorrelations = correlationAnalyzer.Analyze()
		if opts.showCorrelation {
			correlations = limitCorrelationResults(allCorrelations, opts.topN)
			logger.Info().Int("correlated_pairs", len(correlations)).Msg("Correlation analysis complete")
		}
	}

	allFlapping := []analyzer.FlappingResult{}
	flapping := []analyzer.FlappingResult{}
	if opts.showFlapping || opts.showRecommendations {
		flappingAnalyzer := analyzer.NewFlappingAnalyzer(aggregatedHistory, opts.flappingThreshold)
		allFlapping = flappingAnalyzer.Analyze()
		if opts.showFlapping {
			flapping = limitFlappingResults(allFlapping, opts.topN)
			flappingSummary := flappingAnalyzer.GetSummary()
			logger.Info().
				Int("flapping_alerts", flappingSummary.FlappingAlerts).
				Float64("threshold", opts.flappingThreshold).
				Msg("Flapping analysis complete")
		}
	}

	temporal := []analyzer.TemporalResult{}
	if opts.showTemporalPatterns {
		temporalAnalyzer := analyzer.NewTemporalAnalyzer(aggregatedHistory)
		temporal = temporalAnalyzer.AnalyzeTopN(opts.topN)
		logger.Info().Int("temporal_patterns", len(temporal)).Msg("Temporal pattern analysis complete")
	}

	recommendations := []analyzer.Recommendation{}
	if opts.showRecommendations {
		recommendationEngine := analyzer.NewRecommendationEngine()
		recommendations = recommendationEngine.Generate(allFrequency, allFlapping, allCorrelations, allRules)
		logger.Info().Int("recommendations", len(recommendations)).Msg("Recommendation analysis complete")
	}

	if opts.alertmanagerURL != "" {
		ctx, cancel := context.WithTimeout(context.Background(), timeout)
		defer cancel()
		if err := collectAlertmanagerData(ctx, opts.alertmanagerURL, timeout, opts.insecure, logger); err != nil {
			logger.Error().Err(err).Msg("Alertmanager collection failed")
		}
	}

	return &analysisResult{
		stats:           stats,
		topAlerts:       topAlerts,
		allFrequency:    allFrequency,
		flapping:        flapping,
		correlation:     correlations,
		temporal:        temporal,
		recommendations: recommendations,
		history:         aggregatedHistory,
	}, nil
}

func reportAnalysis(result *analysisResult, outputFormat string) error {
	rep := reporter.NewReporter(outputFormat, os.Stdout)
	return rep.ReportCompleteWithInsights(
		result.stats,
		result.topAlerts,
		result.flapping,
		result.correlation,
		result.temporal,
		result.recommendations,
	)
}

func recordAnalysisMetrics(result *analysisResult, command string) {
	if result == nil || result.history == nil {
		return
	}

	toolkitmetrics.CommandExecutions.WithLabelValues(command, "success").Inc()
	toolkitmetrics.ResourcesProcessed.WithLabelValues(command, "alerts").Add(float64(result.history.CountAlerts()))
	toolkitmetrics.SetAlertAnalyzerMetrics(
		result.stats,
		result.topAlerts,
		result.flapping,
		result.correlation,
		result.temporal,
		result.recommendations,
	)
}

func limitFrequencyResults(results []analyzer.FrequencyResult, topN int) []analyzer.FrequencyResult {
	if topN > 0 && topN < len(results) {
		return results[:topN]
	}
	return results
}

func limitCorrelationResults(results []analyzer.CorrelationResult, topN int) []analyzer.CorrelationResult {
	if topN > 0 && topN < len(results) {
		return results[:topN]
	}
	return results
}

func limitFlappingResults(results []analyzer.FlappingResult, topN int) []analyzer.FlappingResult {
	if topN > 0 && topN < len(results) {
		return results[:topN]
	}
	return results
}
