package config

import (
	"github.com/neogan/sre-toolkit/pkg/config"
)

// Config holds the configuration for the config-linter
type Config struct {
	*config.Config // Embed base config

	// Add config-linter specific configuration here
	RulesPath string   `mapstructure:"rules_path"`
	Ignore    []string `mapstructure:"ignore"`
}

// Default creates a new default configuration
func Default() *Config {
	return &Config{
		Config:    config.Default(),
		RulesPath: "",
		Ignore:    []string{},
	}
}
