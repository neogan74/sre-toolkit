// Package main provides the HTTP load testing entry point for chaos engineering.
package main

import (
	"time"

	"github.com/neogan/sre-toolkit/internal/chaos-load/http"
	"github.com/neogan/sre-toolkit/pkg/logging"
	"github.com/spf13/cobra"
)

func newHTTPCmd() *cobra.Command {
	var (
		url         string
		concurrency int
		duration    time.Duration
		requests    int
	)

	cmd := &cobra.Command{
		Use:   "http",
		Short: "Run HTTP load test",
		Long:  "Generates HTTP load against a target URL",
		RunE: func(cmd *cobra.Command, args []string) error {
			logger := logging.GetLogger()
			logger.Info().Str("url", url).Msg("Starting HTTP load test")

			// Initialize worker pool
			pool := http.NewPool(http.PoolConfig{
				TargetURL:   url,
				Concurrency: concurrency,
				Duration:    duration,
				Requests:    requests,
			})

			// Run load test
			if err := pool.Run(); err != nil {
				return err
			}

			return nil
		},
	}

	cmd.Flags().StringVar(&url, "url", "", "Target URL")
	cmd.Flags().IntVar(&concurrency, "concurrency", 10, "Number of concurrent workers")
	cmd.Flags().DurationVar(&duration, "duration", 30*time.Second, "Duration of the test")
	cmd.Flags().IntVar(&requests, "requests", 0, "Total number of requests (0 for unlimited)")

	cmd.MarkFlagRequired("url")

	return cmd
}
