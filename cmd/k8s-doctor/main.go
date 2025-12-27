package main

import (
	"os"

	"github.com/neogan/sre-toolkit/pkg/cli"
	"github.com/neogan/sre-toolkit/pkg/config"
	"github.com/neogan/sre-toolkit/pkg/logging"
	"github.com/neogan/sre-toolkit/pkg/metrics"
	"github.com/spf13/cobra"
)

func main() {
	// Initialize configuration
	cfg := config.Default()

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
		defer metricsServer.Stop()
	}

	// Create root command
	rootCmd := cli.NewRootCmd()
	rootCmd.Use = "k8s-doctor"
	rootCmd.Short = "Kubernetes cluster health checker and diagnostics tool"
	rootCmd.Long = `k8s-doctor is a comprehensive Kubernetes cluster diagnostics tool.
It performs health checks, identifies issues, and provides recommendations
for improving your cluster's reliability and security.`

	// Add subcommands
	rootCmd.AddCommand(newHealthCheckCmd())
	rootCmd.AddCommand(newDiagnosticsCmd())
	rootCmd.AddCommand(newAuditCmd())
	rootCmd.AddCommand(newVersionCmd())

	// Execute
	if err := rootCmd.Execute(); err != nil {
		logger.Error().Err(err).Msg("Command execution failed")
		os.Exit(1)
	}
}

func newHealthCheckCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "healthcheck",
		Short: "Run cluster health checks",
		Long:  "Performs comprehensive health checks on the Kubernetes cluster",
		RunE: func(cmd *cobra.Command, args []string) error {
			logger := logging.GetLogger()
			logger.Info().Msg("Running health checks...")
			// TODO: Implement health check logic
			logger.Info().Msg("Health check completed (not yet implemented)")
			return nil
		},
	}

	return cmd
}

func newDiagnosticsCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "diagnostics",
		Short: "Run cluster diagnostics",
		Long:  "Identifies common problems and issues in the cluster",
		RunE: func(cmd *cobra.Command, args []string) error {
			logger := logging.GetLogger()
			logger.Info().Msg("Running diagnostics...")
			// TODO: Implement diagnostics logic
			logger.Info().Msg("Diagnostics completed (not yet implemented)")
			return nil
		},
	}

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
