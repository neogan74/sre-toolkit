package main

import (
	"context"
	"os"

	"github.com/neogan/sre-toolkit/pkg/cli"
	"github.com/neogan/sre-toolkit/pkg/config"
	"github.com/neogan/sre-toolkit/pkg/logging"
	"github.com/neogan/sre-toolkit/pkg/tracing"
)

func main() {
	// Initialize configuration
	cfg := config.Default()

	// Initialize logging
	logging.Init(cfg.Logging)
	logger := logging.GetLogger()

	// Initialize tracing
	shutdownTracer, err := tracing.InitTracer("chaos-load", *cfg.Tracing)
	if err != nil {
		logger.Error().Err(err).Msg("Failed to initialize tracing")
	} else {
		defer shutdownTracer(context.Background())
	}

	// Create root command
	rootCmd := cli.NewRootCmd()
	rootCmd.Use = "chaos-load"
	rootCmd.Short = "Load testing and chaos engineering tool"
	rootCmd.Long = `chaos-load is a tool for generating load and simulating failure scenarios
in comprehensive tests. It helps verify system resilience and capability.`

	// Add subcommands
	rootCmd.AddCommand(newHTTPCmd())
	rootCmd.AddCommand(newMockCmd())

	// Execute
	if err := rootCmd.Execute(); err != nil {
		logger.Error().Err(err).Msg("Command execution failed")
		os.Exit(1)
	}
}
