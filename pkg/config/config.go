// Package config provides configuration management for the sre-toolkit.
package config

import (
	"github.com/neogan/sre-toolkit/pkg/logging"
	"github.com/neogan/sre-toolkit/pkg/metrics"
	"github.com/neogan/sre-toolkit/pkg/tracing"
)

// Config represents the global application configuration
type Config struct {
	Logging *logging.Config
	Metrics *metrics.Config
	Tracing *tracing.Config
}

// Default returns the default configuration
func Default() *Config {
	return &Config{
		Logging: logging.DefaultConfig(),
		Metrics: metrics.DefaultConfig(),
		Tracing: tracing.DefaultConfig(),
	}
}

// Validate validates the configuration
func (c *Config) Validate() error {
	// Add validation logic here as needed
	return nil
}
