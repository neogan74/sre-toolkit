package logging

import (
	"os"
	"time"

	"github.com/rs/zerolog"
	"github.com/rs/zerolog/log"
)

// Logger is the global logger instance
var Logger zerolog.Logger

// Config holds logging configuration
type Config struct {
	Level      string
	Format     string // "json" or "console"
	TimeFormat string
}

// DefaultConfig returns default logging configuration
func DefaultConfig() *Config {
	return &Config{
		Level:      "info",
		Format:     "console",
		TimeFormat: time.RFC3339,
	}
}

// Init initializes the global logger
func Init(cfg *Config) {
	if cfg == nil {
		cfg = DefaultConfig()
	}

	// Set log level
	level, err := zerolog.ParseLevel(cfg.Level)
	if err != nil {
		level = zerolog.InfoLevel
	}
	zerolog.SetGlobalLevel(level)

	// Set time format
	zerolog.TimeFieldFormat = cfg.TimeFormat

	// Configure output format
	var logger zerolog.Logger
	if cfg.Format == "json" {
		logger = zerolog.New(os.Stdout).With().Timestamp().Logger()
	} else {
		// Console output with colors
		output := zerolog.ConsoleWriter{
			Out:        os.Stdout,
			TimeFormat: time.RFC3339,
		}
		logger = zerolog.New(output).With().Timestamp().Logger()
	}

	Logger = logger
	log.Logger = logger
}

// GetLogger returns the global logger
func GetLogger() zerolog.Logger {
	return Logger
}

// WithComponent returns a logger with component field
func WithComponent(component string) zerolog.Logger {
	return Logger.With().Str("component", component).Logger()
}
