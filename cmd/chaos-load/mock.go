package main

import (
	"time"

	"github.com/neogan/sre-toolkit/internal/chaos-load/mock"
	"github.com/spf13/cobra"
)

func newMockCmd() *cobra.Command {
	var port int
	var errorRate int
	var connectionFailureRate int
	var timeoutRate int
	var timeoutDuration time.Duration
	var latency time.Duration
	var jitter time.Duration

	cmd := &cobra.Command{
		Use:   "mock",
		Short: "Run a chaos mock server",
		Long: `Starts an HTTP server that simulates chaos scenarios.
Useful for testing observability tools and verify client resilience.`,
		RunE: func(cmd *cobra.Command, args []string) error {
			cfg := mock.ServerConfig{
				Port:                  port,
				ErrorRate:             errorRate,
				ConnectionFailureRate: connectionFailureRate,
				TimeoutRate:           timeoutRate,
				TimeoutDuration:       timeoutDuration,
				Latency:               latency,
				Jitter:                jitter,
			}
			if err := cfg.Validate(); err != nil {
				return err
			}

			server := mock.NewServer(cfg)
			return server.Run()
		},
	}

	cmd.Flags().IntVar(&port, "port", 8080, "Port to listen on")
	cmd.Flags().IntVar(&errorRate, "error-rate", 0, "Percentage of requests to fail with 500 (0-100)")
	cmd.Flags().IntVar(&connectionFailureRate, "connection-failure-rate", 0, "Percentage of requests to drop by closing the client connection (0-100)")
	cmd.Flags().IntVar(&timeoutRate, "timeout-rate", 0, "Percentage of requests to delay and return HTTP 504 Gateway Timeout (0-100)")
	cmd.Flags().DurationVar(&timeoutDuration, "timeout-duration", 0, "Duration to wait before returning HTTP 504 for timed-out requests")
	cmd.Flags().DurationVar(&latency, "latency", 0, "Artificial latency to inject (e.g. 100ms)")
	cmd.Flags().DurationVar(&jitter, "jitter", 0, "Random latency variation (+/- duration)")

	return cmd
}
