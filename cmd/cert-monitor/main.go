// Package main provides the entry point for the cert-monitor tool.
package main

import (
	"context"
	"fmt"
	"os"
	"time"

	"github.com/neogan/sre-toolkit/internal/cert-monitor/k8ssecrets"
	"github.com/neogan/sre-toolkit/internal/cert-monitor/notifier"
	"github.com/neogan/sre-toolkit/internal/cert-monitor/reporter"
	"github.com/neogan/sre-toolkit/internal/cert-monitor/scanner"
	"github.com/neogan/sre-toolkit/pkg/k8s"
	"github.com/neogan/sre-toolkit/pkg/logging"
	"github.com/neogan/sre-toolkit/pkg/metrics"
	"github.com/rs/zerolog/log"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"
)

func main() {
	logging.Init(nil)

	rootCmd := &cobra.Command{
		Use:   "cert-monitor",
		Short: "TLS certificate monitoring and expiry alerting",
		Long: `cert-monitor scans TLS certificates for URLs and Kubernetes secrets,
tracks expiry dates, and sends alerts via webhooks.`,
	}

	// Global flags
	rootCmd.PersistentFlags().String("log-level", "info", "Log level (debug, info, warn, error)")
	rootCmd.PersistentFlags().String("output", "table", "Output format (table, json)")
	rootCmd.PersistentFlags().String("config", "", "Config file path")
	rootCmd.PersistentFlags().String("metrics-addr", "", "Address to expose Prometheus metrics (e.g. :9101). Empty = disabled")

	_ = viper.BindPFlag("log_level", rootCmd.PersistentFlags().Lookup("log-level"))
	_ = viper.BindPFlag("output", rootCmd.PersistentFlags().Lookup("output"))
	_ = viper.BindPFlag("metrics_addr", rootCmd.PersistentFlags().Lookup("metrics-addr"))

	rootCmd.AddCommand(
		newScanCmd(),
		newK8sCmd(),
		newWatchCmd(),
		newVersionCmd(),
	)

	if err := rootCmd.Execute(); err != nil {
		log.Error().Err(err).Msg("Command execution failed")
		os.Exit(1)
	}
}

// newScanCmd creates the `scan` subcommand for scanning URLs.
func newScanCmd() *cobra.Command {
	var (
		warnDays    int
		critDays    int
		timeout     time.Duration
		webhookURL  string
		insecure    bool
		concurrency int
	)

	cmd := &cobra.Command{
		Use:   "scan [host...]",
		Short: "Scan TLS certificates for one or more hosts",
		Long: `Connects to each host via TLS and retrieves certificate information.
Accepts hostnames, host:port pairs, or full https:// URLs.

Examples:
  cert-monitor scan example.com
  cert-monitor scan example.com:8443 api.example.com
  cert-monitor scan https://example.com`,
		Args: cobra.MinimumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			logger := logging.GetLogger()
			outputFmt := reporter.Format(viper.GetString("output"))
			rep := reporter.New(outputFmt, os.Stdout)

			cfg := &scanner.Config{
				Timeout:            timeout,
				WarnThreshold:      warnDays,
				CriticalThreshold:  critDays,
				InsecureSkipVerify: insecure,
				Concurrency:        concurrency,
			}

			// Start metrics server if address is configured.
			if addr := viper.GetString("metrics_addr"); addr != "" {
				ms := metrics.NewServer(&metrics.Config{Enabled: true, Address: addr, Path: "/metrics"})
				go func() { _ = ms.Start() }()
				defer func() { _ = ms.Stop() }()
				logger.Info().Str("addr", addr).Msg("Prometheus metrics available at /metrics")
			}

			ctx := cmd.Context()
			logger.Info().Strs("hosts", args).Msg("Scanning certificates")

			start := time.Now()
			results := scanner.ScanURLs(ctx, args, cfg)
			scanDuration := time.Since(start)

			metrics.SetCertMonitorMetrics(results, scanDuration)

			if err := rep.ReportURLScan(results); err != nil {
				return err
			}

			ok, warn, crit, expired, errs := countStatuses(results)
			rep.PrintSummary(len(results), ok, warn, crit, expired, errs)

			// Send webhook if configured
			if webhookURL != "" {
				n := notifier.NewWebhookNotifier(webhookURL, timeout)
				if err := n.Notify(ctx, results); err != nil {
					logger.Error().Err(err).Msg("Failed to send webhook notification")
				} else {
					logger.Info().Str("url", webhookURL).Msg("Webhook notification sent")
				}
			}

			// Exit with error if any critical/expired/error
			if crit+expired+errs > 0 {
				return fmt.Errorf("%d critical, %d expired, %d error certificates found", crit, expired, errs)
			}
			return nil
		},
	}

	cmd.Flags().IntVar(&warnDays, "warn-days", 30, "Days before expiry to trigger WARNING")
	cmd.Flags().IntVar(&critDays, "crit-days", 7, "Days before expiry to trigger CRITICAL")
	cmd.Flags().DurationVar(&timeout, "timeout", 10*time.Second, "Connection timeout per host")
	cmd.Flags().StringVar(&webhookURL, "webhook", "", "Webhook URL to send alerts to")
	cmd.Flags().BoolVar(&insecure, "insecure", false, "Skip TLS verification (for self-signed certs)")
	cmd.Flags().IntVar(&concurrency, "concurrency", 10, "Number of concurrent scans")

	return cmd
}

// newK8sCmd creates the `k8s` subcommand for scanning Kubernetes TLS secrets.
func newK8sCmd() *cobra.Command {
	var (
		kubeconfig string
		namespace  string
		warnDays   int
		critDays   int
		webhookURL string
		timeout    time.Duration
	)

	cmd := &cobra.Command{
		Use:   "k8s",
		Short: "Scan TLS certificates in Kubernetes secrets",
		Long: `Lists all Kubernetes secrets of type kubernetes.io/tls and checks
their certificates for expiry.

Examples:
  cert-monitor k8s
  cert-monitor k8s --namespace production
  cert-monitor k8s --kubeconfig ~/.kube/config --warn-days 60`,
		RunE: func(cmd *cobra.Command, args []string) error {
			logger := logging.GetLogger()
			outputFmt := reporter.Format(viper.GetString("output"))
			rep := reporter.New(outputFmt, os.Stdout)

			client, err := k8s.NewClient(&k8s.Config{Kubeconfig: kubeconfig})
			if err != nil {
				return fmt.Errorf("creating kubernetes client: %w", err)
			}

			ctx, cancel := context.WithTimeout(cmd.Context(), 30*time.Second)
			defer cancel()

			if err := client.Ping(ctx); err != nil {
				return fmt.Errorf("connecting to cluster: %w", err)
			}

			cfg := &scanner.Config{
				WarnThreshold:     warnDays,
				CriticalThreshold: critDays,
				Timeout:           timeout,
				Concurrency:       10,
			}

			logger.Info().Str("namespace", namespaceOrAll(namespace)).Msg("Scanning K8s TLS secrets")

			secretInfos, err := k8ssecrets.ScanSecrets(ctx, client.Clientset(), namespace, cfg)
			if err != nil {
				return fmt.Errorf("scanning secrets: %w", err)
			}

			if len(secretInfos) == 0 {
				fmt.Println("No TLS secrets found.")
				return nil
			}

			// Convert to CertRows for reporter
			rows := make([]reporter.CertRow, 0, len(secretInfos))
			var urlResults []*scanner.CertInfo
			for _, si := range secretInfos {
				expires := ""
				if !si.NotAfter.IsZero() {
					expires = si.NotAfter.Format("2006-01-02")
				}
				rows = append(rows, reporter.CertRow{
					Source:    si.Host,
					Subject:   si.Subject,
					Issuer:    si.Issuer,
					NotAfter:  expires,
					DaysLeft:  si.DaysLeft,
					Status:    string(si.Status),
					Error:     si.Error,
					Namespace: si.Namespace,
					Secret:    si.SecretName,
				})
				urlResults = append(urlResults, si.CertInfo)
			}

			if err := rep.ReportCertList(rows); err != nil {
				return err
			}

			ok, warn, crit, expired, errs := countStatuses(urlResults)
			rep.PrintSummary(len(rows), ok, warn, crit, expired, errs)

			// Send webhook alerts
			if webhookURL != "" {
				n := notifier.NewWebhookNotifier(webhookURL, timeout)
				if err := n.Notify(ctx, urlResults); err != nil {
					logger.Error().Err(err).Msg("Failed to send webhook notification")
				} else {
					logger.Info().Str("url", webhookURL).Msg("Webhook notification sent")
				}
			}

			if crit+expired+errs > 0 {
				return fmt.Errorf("%d critical, %d expired, %d error certificates found", crit, expired, errs)
			}
			return nil
		},
	}

	cmd.Flags().StringVar(&kubeconfig, "kubeconfig", "", "Path to kubeconfig file")
	cmd.Flags().StringVarP(&namespace, "namespace", "n", "", "Namespace to scan (empty = all namespaces)")
	cmd.Flags().IntVar(&warnDays, "warn-days", 30, "Days before expiry to trigger WARNING")
	cmd.Flags().IntVar(&critDays, "crit-days", 7, "Days before expiry to trigger CRITICAL")
	cmd.Flags().StringVar(&webhookURL, "webhook", "", "Webhook URL to send alerts to")
	cmd.Flags().DurationVar(&timeout, "timeout", 10*time.Second, "Connection timeout")

	return cmd
}

// newWatchCmd creates the `watch` subcommand for continuous monitoring.
func newWatchCmd() *cobra.Command {
	var (
		interval    time.Duration
		warnDays    int
		critDays    int
		timeout     time.Duration
		webhookURL  string
		insecure    bool
		concurrency int
	)

	cmd := &cobra.Command{
		Use:   "watch [host...]",
		Short: "Continuously monitor TLS certificates",
		Long: `Runs certificate scans on a configurable interval and sends
webhook alerts when issues are detected.

Examples:
  cert-monitor watch --interval 6h example.com api.example.com
  cert-monitor watch --interval 1h --webhook https://hooks.example.com/alerts example.com`,
		Args: cobra.MinimumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			logger := logging.GetLogger()
			outputFmt := reporter.Format(viper.GetString("output"))

			cfg := &scanner.Config{
				Timeout:            timeout,
				WarnThreshold:      warnDays,
				CriticalThreshold:  critDays,
				InsecureSkipVerify: insecure,
				Concurrency:        concurrency,
			}

			// Start metrics server — essential for watch mode so Prometheus can scrape.
			if addr := viper.GetString("metrics_addr"); addr != "" {
				ms := metrics.NewServer(&metrics.Config{Enabled: true, Address: addr, Path: "/metrics"})
				go func() { _ = ms.Start() }()
				defer func() { _ = ms.Stop() }()
				logger.Info().Str("addr", addr).Msg("Prometheus metrics available at /metrics")
			}

			logger.Info().
				Strs("hosts", args).
				Dur("interval", interval).
				Msg("Starting certificate watch")

			ctx := cmd.Context()
			runScan := func() {
				rep := reporter.New(outputFmt, os.Stdout)
				start := time.Now()
				results := scanner.ScanURLs(ctx, args, cfg)
				metrics.SetCertMonitorMetrics(results, time.Since(start))
				_ = rep.ReportURLScan(results)
				ok, warn, crit, expired, errs := countStatuses(results)
				rep.PrintSummary(len(results), ok, warn, crit, expired, errs)

				if webhookURL != "" {
					n := notifier.NewWebhookNotifier(webhookURL, timeout)
					if err := n.Notify(ctx, results); err != nil {
						logger.Error().Err(err).Msg("Webhook notification failed")
					}
				}
			}

			// Run immediately, then on interval
			runScan()
			ticker := time.NewTicker(interval)
			defer ticker.Stop()
			for {
				select {
				case <-ctx.Done():
					return nil
				case <-ticker.C:
					runScan()
				}
			}
		},
	}

	cmd.Flags().DurationVar(&interval, "interval", 6*time.Hour, "Scan interval")
	cmd.Flags().IntVar(&warnDays, "warn-days", 30, "Days before expiry to trigger WARNING")
	cmd.Flags().IntVar(&critDays, "crit-days", 7, "Days before expiry to trigger CRITICAL")
	cmd.Flags().DurationVar(&timeout, "timeout", 10*time.Second, "Connection timeout per host")
	cmd.Flags().StringVar(&webhookURL, "webhook", "", "Webhook URL to send alerts to")
	cmd.Flags().BoolVar(&insecure, "insecure", false, "Skip TLS verification")
	cmd.Flags().IntVar(&concurrency, "concurrency", 10, "Number of concurrent scans")

	return cmd
}

func newVersionCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "version",
		Short: "Print version information",
		Run: func(cmd *cobra.Command, args []string) {
			fmt.Println("cert-monitor version 0.3.0")
		},
	}
}

func countStatuses(results []*scanner.CertInfo) (ok, warn, crit, expired, errs int) {
	for _, r := range results {
		switch r.Status {
		case scanner.StatusOK:
			ok++
		case scanner.StatusWarning:
			warn++
		case scanner.StatusCritical:
			crit++
		case scanner.StatusExpired:
			expired++
		case scanner.StatusError:
			errs++
		}
	}
	return
}

func namespaceOrAll(ns string) string {
	if ns == "" {
		return "all namespaces"
	}
	return ns
}
