// Package alertmanager provides a client for interacting with the Alertmanager API.
package alertmanager

import (
	"context"
	"crypto/tls"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"time"

	"github.com/rs/zerolog"
)

// Config holds the configuration for the Alertmanager client
type Config struct {
	URL      string        // Alertmanager server URL
	Username string        // Basic auth username (optional)
	Password string        // Basic auth password (optional)
	Timeout  time.Duration // Request timeout
	Insecure bool          // Skip TLS verification
}

// Client is a simple HTTP client for Alertmanager API v2
type Client struct {
	baseURL *url.URL
	client  *http.Client
	config  *Config
	logger  *zerolog.Logger
}

// NewClient creates a new Alertmanager client
func NewClient(cfg *Config, logger *zerolog.Logger) (*Client, error) {
	if cfg.URL == "" {
		return nil, fmt.Errorf("alertmanager URL is required")
	}

	u, err := url.Parse(cfg.URL)
	if err != nil {
		return nil, fmt.Errorf("invalid alertmanager URL: %w", err)
	}

	// Set default timeout if not specified
	if cfg.Timeout == 0 {
		cfg.Timeout = 30 * time.Second
	}

	// Create HTTP client with custom transport
	httpClient := &http.Client{
		Timeout: cfg.Timeout,
		Transport: &http.Transport{
			TLSClientConfig: &tls.Config{
				InsecureSkipVerify: cfg.Insecure, // #nosec G402
			},
		},
	}

	return &Client{
		baseURL: u,
		client:  httpClient,
		config:  cfg,
		logger:  logger,
	}, nil
}

// Ping checks connectivity to Alertmanager
func (c *Client) Ping(ctx context.Context) error {
	c.logger.Debug().Msg("Pinging Alertmanager server")

	// Helper to check status endpoint
	statusURL := c.baseURL.JoinPath("api/v2/status")

	req, err := http.NewRequestWithContext(ctx, "GET", statusURL.String(), http.NoBody)
	if err != nil {
		return fmt.Errorf("failed to create request: %w", err)
	}

	if c.config.Username != "" && c.config.Password != "" {
		req.SetBasicAuth(c.config.Username, c.config.Password)
	}

	resp, err := c.client.Do(req)
	if err != nil {
		return fmt.Errorf("failed to connect to alertmanager: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("alertmanager returned status: %s", resp.Status)
	}

	c.logger.Info().Str("url", c.config.URL).Msg("Successfully connected to Alertmanager")
	return nil
}

// Alert represents an Alertmanager alert
type Alert struct {
	Annotations  map[string]string `json:"annotations"`
	EndsAt       time.Time         `json:"endsAt"`
	Fingerprint  string            `json:"fingerprint"`
	Receivers    []map[string]any  `json:"receivers"`
	StartsAt     time.Time         `json:"startsAt"`
	Status       map[string]string `json:"status"`
	UpdatedAt    time.Time         `json:"updatedAt"`
	GeneratorURL string            `json:"generatorURL"`
	Labels       map[string]string `json:"labels"`
}

// ListAlerts fetches current alerts from Alertmanager
func (c *Client) ListAlerts(ctx context.Context, filter []string) ([]Alert, error) {
	u := c.baseURL.JoinPath("api/v2/alerts")

	q := u.Query()
	q.Set("active", "true")
	q.Set("silenced", "false")
	q.Set("inhibited", "false")
	q.Set("unprocessed", "false")

	for _, f := range filter {
		q.Add("filter", f)
	}
	u.RawQuery = q.Encode()

	c.logger.Debug().Str("url", u.String()).Msg("Fetching alerts from Alertmanager")

	req, err := http.NewRequestWithContext(ctx, "GET", u.String(), http.NoBody)
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	if c.config.Username != "" && c.config.Password != "" {
		req.SetBasicAuth(c.config.Username, c.config.Password)
	}

	resp, err := c.client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("request failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("unexpected status code: %d", resp.StatusCode)
	}

	var alerts []Alert
	if err := json.NewDecoder(resp.Body).Decode(&alerts); err != nil {
		return nil, fmt.Errorf("failed to decode response: %w", err)
	}

	return alerts, nil
}
