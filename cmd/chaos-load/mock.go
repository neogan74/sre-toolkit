package main

import (
	"time"

	"github.com/neogan/sre-toolkit/internal/chaos-load/mock"
	"github.com/spf13/cobra"
)

func newMockCmd() *cobra.Command {
	var port int
	var errorRate int
	var latency time.Duration

	cmd := &cobra.Command{
		Use:   "mock",
		Short: "Run a chaos mock server",
		Long: `Starts an HTTP server that simulates chaos scenarios.
Useful for testing observability tools and verify client resilience.`,
		RunE: func(cmd *cobra.Command, args []string) error {
			cfg := mock.ServerConfig{
				Port:      port,
				ErrorRate: errorRate,
				Latency:   latency,
			}
			server := mock.NewServer(cfg)
			return server.Run()
		},
	}

	cmd.Flags().IntVar(&port, "port", 8080, "Port to listen on")
	cmd.Flags().IntVar(&errorRate, "error-rate", 0, "Percentage of requests to fail with 500 (0-100)")
	cmd.Flags().DurationVar(&latency, "latency", 0, "Artificial latency to inject (e.g. 100ms)")

	return cmd
}
