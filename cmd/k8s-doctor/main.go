// Package main provides the entry point for the k8s-doctor tool.
package main

import (
	"context"
	"os"
	"time"

	"github.com/neogan/sre-toolkit/internal/k8s-doctor/diagnostics"
	"github.com/neogan/sre-toolkit/internal/k8s-doctor/healthcheck"
	"github.com/neogan/sre-toolkit/internal/k8s-doctor/reporter"
	"github.com/neogan/sre-toolkit/pkg/cli"
	"github.com/neogan/sre-toolkit/pkg/config"
	"github.com/neogan/sre-toolkit/pkg/k8s"
	"github.com/neogan/sre-toolkit/pkg/logging"
	"github.com/neogan/sre-toolkit/pkg/metrics"
	"github.com/neogan/sre-toolkit/pkg/tracing"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"
)

func main() {
	// Initialize configuration
	cfg := config.Default()
	var shutdownTracer func(context.Context) error

	// Create root command
	rootCmd := cli.NewRootCmd()
	rootCmd.Use = "k8s-doctor"
	rootCmd.Short = "Kubernetes cluster health checker and diagnostics tool"
	rootCmd.Long = `k8s-doctor is a comprehensive Kubernetes cluster diagnostics tool.
It performs health checks, identifies issues, and provides recommendations
for improving your cluster's reliability and security.`

	rootCmd.PersistentPreRunE = func(cmd *cobra.Command, args []string) error {
		// Load config from viper
		if err := viper.Unmarshal(cfg); err != nil {
			return err
		}

		// Initialize logging
		logging.Init(cfg.Logging)
		logger := logging.GetLogger()

		// Initialize metrics server
		metricsServer := metrics.NewServer(cfg.Metrics)
		if cfg.Metrics.Enabled {
			go func() {
				if err := metricsServer.Start(); err != nil {
					logger.Error().Err(err).Msg("Failed to start metrics server")
				}
			}()
			// Note: Metrics server stop is not easily handled in PreRun/PostRun split without globals
			// For now we let it run until exit.
		}

		// Initialize tracing
		var err error
		shutdownTracer, err = tracing.InitTracer("k8s-doctor", *cfg.Tracing)
		if err != nil {
			logger.Error().Err(err).Msg("Failed to initialize tracing")
		}

		return nil
	}

	rootCmd.PersistentPostRunE = func(cmd *cobra.Command, args []string) error {
		if shutdownTracer != nil {
			return shutdownTracer(context.Background())
		}
		return nil
	}

	// Add subcommands
	rootCmd.AddCommand(newHealthCheckCmd())
	rootCmd.AddCommand(newDiagnosticsCmd())
	rootCmd.AddCommand(newAuditCmd())
	rootCmd.AddCommand(newVersionCmd())

	// Execute
	if err := rootCmd.Execute(); err != nil {
		l := logging.GetLogger()
		l.Error().Err(err).Msg("Command execution failed")
		// Don't use os.Exit here to allow deferred cleanup
		return
	}
}

func newHealthCheckCmd() *cobra.Command {
	var (
		kubeconfig string
		namespace  string
		output     string
		timeout    time.Duration
	)

	cmd := &cobra.Command{
		Use:   "healthcheck",
		Short: "Run cluster health checks",
		Long:  "Performs comprehensive health checks on the Kubernetes cluster",
		RunE: func(cmd *cobra.Command, args []string) error {
			logger := logging.GetLogger()
			logger.Info().Msg("Running health checks...")

			// Create Kubernetes client
			client, err := k8s.NewClient(&k8s.Config{
				Kubeconfig: kubeconfig,
			})
			if err != nil {
				logger.Error().Err(err).Msg("Failed to create Kubernetes client")
				return err
			}

			// Create context with timeout
			ctx, cancel := context.WithTimeout(context.Background(), timeout)
			defer cancel()

			// Check cluster connectivity
			if err := client.Ping(ctx); err != nil {
				logger.Error().Err(err).Msg("Failed to connect to cluster")
				return err
			}

			version, err := client.ServerVersion(ctx)
			if err != nil {
				logger.Warn().Err(err).Msg("Could not get server version")
			}
			logger.Info().Str("version", version).Msg("Connected to cluster")

			// Create reporter
			format := reporter.FormatTable
			if output == "json" {
				format = reporter.FormatJSON
			}
			rep := reporter.NewReporter(format, os.Stdout)

			// Check nodes
			logger.Info().Msg("Checking nodes...")
			nodes, err := healthcheck.CheckNodes(ctx, client.Clientset())
			if err != nil {
				logger.Error().Err(err).Msg("Failed to check nodes")
				return err
			}
			logger.Info().Int("count", len(nodes)).Msg("Nodes checked")

			if output == "table" {
				logger.Info().Msg("\n=== Node Health ===")
			}
			if err := rep.ReportNodeHealth(nodes); err != nil {
				return err
			}

			// Check pods
			logger.Info().Msg("Checking pods...")
			pods, err := healthcheck.CheckPods(ctx, client.Clientset(), namespace)
			if err != nil {
				logger.Error().Err(err).Msg("Failed to check pods")
				return err
			}
			logger.Info().Int("total", pods.Total).Int("problems", len(pods.ProblemPods)).Msg("Pods checked")

			if err := rep.ReportPodHealth(pods); err != nil {
				return err
			}

			// Check components
			logger.Info().Msg("Checking components...")
			components, err := healthcheck.CheckComponents(ctx, client.Clientset())
			if err != nil {
				logger.Error().Err(err).Msg("Failed to check components")
				return err
			}
			logger.Info().Int("count", len(components)).Msg("Components checked")

			if output == "table" {
				logger.Info().Msg("\n=== Component Health ===")
			}
			if err := rep.ReportComponentHealth(components); err != nil {
				return err
			}

			logger.Info().Msg("Health check completed successfully")
			return nil
		},
	}

	cmd.Flags().StringVar(&kubeconfig, "kubeconfig", "", "Path to kubeconfig file")
	cmd.Flags().StringVarP(&namespace, "namespace", "n", "", "Namespace to check (empty for all)")
	cmd.Flags().StringVarP(&output, "output", "o", "table", "Output format (table, json)")
	cmd.Flags().DurationVar(&timeout, "timeout", 30*time.Second, "Request timeout")

	return cmd
}

func newDiagnosticsCmd() *cobra.Command {
	var (
		kubeconfig string
		namespace  string
		output     string
		timeout    time.Duration
	)

	cmd := &cobra.Command{
		Use:   "diagnostics",
		Short: "Run cluster diagnostics",
		Long:  "Identifies common problems and issues in the cluster",
		RunE: func(cmd *cobra.Command, args []string) error {
			logger := logging.GetLogger()
			logger.Info().Msg("Running diagnostics...")

			// Create Kubernetes client
			client, err := k8s.NewClient(&k8s.Config{
				Kubeconfig: kubeconfig,
			})
			if err != nil {
				logger.Error().Err(err).Msg("Failed to create Kubernetes client")
				return err
			}

			// Create context with timeout
			ctx, cancel := context.WithTimeout(context.Background(), timeout)
			defer cancel()

			// Check cluster connectivity
			if err := client.Ping(ctx); err != nil {
				logger.Error().Err(err).Msg("Failed to connect to cluster")
				return err
			}

			version, err := client.ServerVersion(ctx)
			if err != nil {
				logger.Warn().Err(err).Msg("Could not get server version")
			}
			logger.Info().Str("version", version).Msg("Connected to cluster")

			// Run diagnostics
			logger.Info().Msg("Analyzing cluster...")
			result, err := diagnostics.RunDiagnostics(ctx, client.Clientset(), namespace)
			if err != nil {
				logger.Error().Err(err).Msg("Failed to run diagnostics")
				return err
			}

			// Create reporter
			format := reporter.FormatTable
			if output == "json" {
				format = reporter.FormatJSON
			}
			rep := reporter.NewReporter(format, os.Stdout)

			// Report results
			if err := rep.ReportDiagnostics(result); err != nil {
				return err
			}

			// Log summary
			logger.Info().
				Int("total_issues", result.Summary.TotalIssues).
				Int("critical", result.Summary.CriticalCount).
				Int("warning", result.Summary.WarningCount).
				Int("info", result.Summary.InfoCount).
				Msg("Diagnostics completed")

			// Exit with error code if critical issues found
			if result.Summary.CriticalCount > 0 {
				os.Exit(1)
			}

			return nil
		},
	}

	cmd.Flags().StringVar(&kubeconfig, "kubeconfig", "", "Path to kubeconfig file")
	cmd.Flags().StringVarP(&namespace, "namespace", "n", "", "Namespace to check (empty for all)")
	cmd.Flags().StringVarP(&output, "output", "o", "table", "Output format (table, json)")
	cmd.Flags().DurationVar(&timeout, "timeout", 30*time.Second, "Request timeout")

	return cmd
}

func newAuditCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "audit",
		Short: "Run security and best practices audit",
		Long:  "Audits the cluster for security issues and best practices violations",
		RunE: func(cmd *cobra.Command, args []string) error {
			logger := logging.GetLogger()
			logger.Info().Msg("Running audit...")
			// TODO: Implement audit logic
			logger.Info().Msg("Audit completed (not yet implemented)")
			return nil
		},
	}

	return cmd
}

func newVersionCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "version",
		Short: "Print version information",
		Run: func(cmd *cobra.Command, args []string) {
			logger := logging.GetLogger()
			logger.Info().Msg("k8s-doctor version 0.1.0")
		},
	}

	return cmd
}
