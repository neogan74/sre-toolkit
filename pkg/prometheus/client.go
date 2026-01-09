// Package prometheus provides a wrapper around the Prometheus API client.
package prometheus

import (
	"context"
	"crypto/tls"
	"fmt"
	"net/http"
	"time"

	"github.com/prometheus/client_golang/api"
	v1 "github.com/prometheus/client_golang/api/prometheus/v1"
	"github.com/prometheus/common/model"
	"github.com/rs/zerolog"
)

// Config holds the configuration for the Prometheus client
type Config struct {
	URL      string        // Prometheus server URL
	Username string        // Basic auth username (optional)
	Password string        // Basic auth password (optional)
	Timeout  time.Duration // Request timeout
	Insecure bool          // Skip TLS verification
}

// Client wraps the Prometheus API client
type Client struct {
	api    v1.API
	config *Config
	logger *zerolog.Logger
}

// NewClient creates a new Prometheus client
func NewClient(cfg *Config, logger *zerolog.Logger) (*Client, error) {
	if cfg.URL == "" {
		return nil, fmt.Errorf("prometheus URL is required")
	}

	// Set default timeout if not specified
	if cfg.Timeout == 0 {
		cfg.Timeout = 30 * time.Second
	}

	// Create HTTP client with custom transport
	httpClient := &http.Client{
		Transport: &http.Transport{
			TLSClientConfig: &tls.Config{
				InsecureSkipVerify: cfg.Insecure, // #nosec G402
			},
		},
	}

	// Add basic auth if provided
	var roundTripper = httpClient.Transport
	if cfg.Username != "" && cfg.Password != "" {
		roundTripper = &basicAuthRoundTripper{
			username: cfg.Username,
			password: cfg.Password,
			next:     roundTripper,
		}
	}

	// Create Prometheus API client
	apiClient, err := api.NewClient(api.Config{
		Address:      cfg.URL,
		RoundTripper: roundTripper,
	})
	if err != nil {
		return nil, fmt.Errorf("failed to create prometheus client: %w", err)
	}

	return &Client{
		api:    v1.NewAPI(apiClient),
		config: cfg,
		logger: logger,
	}, nil
}

// Query executes an instant query
func (c *Client) Query(ctx context.Context, query string, ts time.Time) (model.Value, error) {
	c.logger.Debug().
		Str("query", query).
		Time("time", ts).
		Msg("Executing Prometheus query")

	result, warnings, err := c.api.Query(ctx, query, ts)
	if err != nil {
		c.logger.Error().Err(err).Str("query", query).Msg("Query failed")
		return nil, fmt.Errorf("query failed: %w", err)
	}

	if len(warnings) > 0 {
		c.logger.Warn().Strs("warnings", warnings).Msg("Query returned warnings")
	}

	c.logger.Debug().
		Str("result_type", result.Type().String()).
		Msg("Query completed successfully")

	return result, nil
}

// QueryRange executes a range query
func (c *Client) QueryRange(ctx context.Context, query string, r v1.Range) (model.Value, error) {
	c.logger.Debug().
		Str("query", query).
		Time("start", r.Start).
		Time("end", r.End).
		Dur("step", r.Step).
		Msg("Executing Prometheus range query")

	result, warnings, err := c.api.QueryRange(ctx, query, r)
	if err != nil {
		c.logger.Error().Err(err).Str("query", query).Msg("Range query failed")
		return nil, fmt.Errorf("range query failed: %w", err)
	}

	if len(warnings) > 0 {
		c.logger.Warn().Strs("warnings", warnings).Msg("Range query returned warnings")
	}

	c.logger.Debug().
		Str("result_type", result.Type().String()).
		Msg("Range query completed successfully")

	return result, nil
}

// LabelValues returns all possible values for a label
func (c *Client) LabelValues(ctx context.Context, label string) (model.LabelValues, error) {
	c.logger.Debug().Str("label", label).Msg("Fetching label values")

	values, warnings, err := c.api.LabelValues(ctx, label, nil, time.Now().Add(-24*time.Hour), time.Now())
	if err != nil {
		return nil, fmt.Errorf("failed to get label values: %w", err)
	}

	if len(warnings) > 0 {
		c.logger.Warn().Strs("warnings", warnings).Msg("LabelValues returned warnings")
	}

	return values, nil
}

// Ping checks connectivity to Prometheus
func (c *Client) Ping(ctx context.Context) error {
	c.logger.Debug().Msg("Pinging Prometheus server")

	// Try a simple query to check connectivity
	_, err := c.Query(ctx, "up", time.Now())
	if err != nil {
		return fmt.Errorf("prometheus ping failed: %w", err)
	}

	c.logger.Info().Str("url", c.config.URL).Msg("Successfully connected to Prometheus")
	return nil
}

// BuildInfo returns Prometheus build information
func (c *Client) BuildInfo(ctx context.Context) (v1.BuildinfoResult, error) {
	return c.api.Buildinfo(ctx)
}

// basicAuthRoundTripper adds basic authentication to HTTP requests
type basicAuthRoundTripper struct {
	username string
	password string
	next     http.RoundTripper
}

func (rt *basicAuthRoundTripper) RoundTrip(req *http.Request) (*http.Response, error) {
	req.SetBasicAuth(rt.username, rt.password)
	return rt.next.RoundTrip(req)
}
