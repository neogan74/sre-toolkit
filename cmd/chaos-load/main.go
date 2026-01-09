package main

import (
	"os"

	"github.com/neogan/sre-toolkit/pkg/cli"
	"github.com/neogan/sre-toolkit/pkg/config"
	"github.com/neogan/sre-toolkit/pkg/logging"
)

func main() {
	// Initialize configuration
	cfg := config.Default()

	// Initialize logging
	logging.Init(cfg.Logging)
	logger := logging.GetLogger()

	// Create root command
	rootCmd := cli.NewRootCmd()
	rootCmd.Use = "chaos-load"
	rootCmd.Short = "Load testing and chaos engineering tool"
	rootCmd.Long = `chaos-load is a tool for generating load and simulating failure scenarios
in comprehensive tests. It helps verify system resilience and capability.`

	// Add subcommands
	rootCmd.AddCommand(newHTTPCmd())

	// Execute
	if err := rootCmd.Execute(); err != nil {
		logger.Error().Err(err).Msg("Command execution failed")
		os.Exit(1)
	}
}
