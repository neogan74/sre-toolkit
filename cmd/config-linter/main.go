package main

import (
	"github.com/neogan/sre-toolkit/internal/config-linter/cli"
	"github.com/neogan/sre-toolkit/internal/config-linter/config"
	"github.com/neogan/sre-toolkit/pkg/logging"
	"github.com/neogan/sre-toolkit/pkg/metrics"
)

func main() {
	// Initialize configuration
	cfg := config.Default()

	// Initialize logging
	logging.Init(cfg.Logging)
	logger := logging.GetLogger()

	// Initialize metrics server (if needed in future)
	if cfg.Metrics.Enabled {
		metricsServer := metrics.NewServer(cfg.Metrics)
		go func() {
			if err := metricsServer.Start(); err != nil {
				logger.Error().Err(err).Msg("Failed to start metrics server")
			}
		}()
		defer metricsServer.Stop()
	}

	// Execute CLI
	if err := cli.Execute(); err != nil {
		// Logger error already handled in Execute
		// Return without os.Exit to allow defers to run
		return
	}
}
