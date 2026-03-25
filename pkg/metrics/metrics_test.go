package metrics

import (
	"testing"
	"time"

	"github.com/neogan/sre-toolkit/internal/alert-analyzer/analyzer"
	"github.com/prometheus/client_golang/prometheus/testutil"
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

func TestSetAlertAnalyzerMetrics(t *testing.T) {
	SetAlertAnalyzerMetrics(
		analyzer.SummaryStats{
			TotalAlerts:     10,
			UniqueAlerts:    4,
			TotalFirings:    20,
			AvgDuration:     2 * time.Minute,
			TotalFiringTime: 40 * time.Minute,
		},
		[]analyzer.FrequencyResult{
			{AlertName: "HighCPU", Severity: "critical", FiringCount: 7},
		},
		[]analyzer.FlappingResult{
			{AlertName: "HighCPU", Severity: "critical", FlappingScore: 5.5, IsFlapping: true},
		},
		[]analyzer.CorrelationResult{
			{AlertA: "HighCPU", AlertB: "HighMemory", CorrelationScore: 0.8},
		},
		[]analyzer.TemporalResult{
			{AlertName: "HighCPU", Severity: "critical", PeakWeekday: "Monday", PeakHour: 10, BusinessHoursRatio: 0.75},
		},
		[]analyzer.Recommendation{
			{Category: "review", Priority: "high"},
			{Category: "review", Priority: "high"},
		},
	)

	if got := testutil.ToFloat64(AlertAnalyzerSummary.WithLabelValues("total_firings")); got != 20 {
		t.Fatalf("expected total firings 20, got %v", got)
	}

	if got := testutil.ToFloat64(AlertAnalyzerTopAlertFirings.WithLabelValues("HighCPU", "critical")); got != 7 {
		t.Fatalf("expected top alert firing count 7, got %v", got)
	}

	if got := testutil.ToFloat64(AlertAnalyzerRecommendationTotal.WithLabelValues("review", "high")); got != 2 {
		t.Fatalf("expected recommendation count 2, got %v", got)
	}
}
