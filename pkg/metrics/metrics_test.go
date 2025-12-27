package metrics

import (
	"testing"
)

func TestDefaultConfig(t *testing.T) {
	cfg := DefaultConfig()
	if cfg == nil {
		t.Fatal("DefaultConfig returned nil")
	}

	if cfg.Enabled {
		t.Error("Expected metrics to be disabled by default")
	}

	if cfg.Address != ":9090" {
		t.Errorf("Expected address ':9090', got '%s'", cfg.Address)
	}

	if cfg.Path != "/metrics" {
		t.Errorf("Expected path '/metrics', got '%s'", cfg.Path)
	}
}

func TestNewServer(t *testing.T) {
	cfg := &Config{
		Enabled: false,
		Address: ":9090",
		Path:    "/metrics",
	}

	server := NewServer(cfg)
	if server == nil {
		t.Fatal("NewServer returned nil")
	}

	if server.config.Address != ":9090" {
		t.Errorf("Expected address ':9090', got '%s'", server.config.Address)
	}
}

func TestServerStartStop(t *testing.T) {
	cfg := &Config{
		Enabled: false, // Disabled to avoid port conflicts
		Address: ":9091",
		Path:    "/metrics",
	}

	server := NewServer(cfg)

	// Should not error when disabled
	if err := server.Start(); err != nil {
		t.Errorf("Start() failed: %v", err)
	}

	if err := server.Stop(); err != nil {
		t.Errorf("Stop() failed: %v", err)
	}
}
