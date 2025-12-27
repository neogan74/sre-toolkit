package logging

import (
	"testing"

	"github.com/rs/zerolog"
)

func TestDefaultConfig(t *testing.T) {
	cfg := DefaultConfig()
	if cfg == nil {
		t.Fatal("DefaultConfig returned nil")
	}

	if cfg.Level != "info" {
		t.Errorf("Expected level 'info', got '%s'", cfg.Level)
	}

	if cfg.Format != "console" {
		t.Errorf("Expected format 'console', got '%s'", cfg.Format)
	}
}

func TestInit(t *testing.T) {
	cfg := &Config{
		Level:  "info",
		Format: "json",
	}

	Init(cfg)

	// Check that global level is set correctly
	if zerolog.GlobalLevel() != zerolog.InfoLevel {
		t.Errorf("Expected global info level, got %v", zerolog.GlobalLevel())
	}
}

func TestWithComponent(t *testing.T) {
	Init(DefaultConfig())
	logger := WithComponent("test-component")

	// Just verify it doesn't panic
	logger.Info().Msg("test message")
}
