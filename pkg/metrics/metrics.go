// Package metrics provides Prometheus metrics collection and a metrics server.
package metrics

import (
	"net/http"
	"time"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"
	"github.com/prometheus/client_golang/prometheus/promhttp"
)

var (
	// CommandExecutions tracks CLI command executions
	CommandExecutions = promauto.NewCounterVec(
		prometheus.CounterOpts{
			Name: "sre_toolkit_command_executions_total",
			Help: "Total number of command executions",
		},
		[]string{"command", "status"},
	)

	// CommandDuration tracks command execution duration
	CommandDuration = promauto.NewHistogramVec(
		prometheus.HistogramOpts{
			Name:    "sre_toolkit_command_duration_seconds",
			Help:    "Command execution duration in seconds",
			Buckets: prometheus.DefBuckets,
		},
		[]string{"command"},
	)

	// ResourcesProcessed tracks resources processed by commands
	ResourcesProcessed = promauto.NewCounterVec(
		prometheus.CounterOpts{
			Name: "sre_toolkit_resources_processed_total",
			Help: "Total number of resources processed",
		},
		[]string{"command", "resource_type"},
	)

	// Errors tracks errors by type
	Errors = promauto.NewCounterVec(
		prometheus.CounterOpts{
			Name: "sre_toolkit_errors_total",
			Help: "Total number of errors",
		},
		[]string{"command", "error_type"},
	)
)

// Config holds metrics server configuration
type Config struct {
	Enabled bool
	Address string
	Path    string
}

// DefaultConfig returns default metrics configuration
func DefaultConfig() *Config {
	return &Config{
		Enabled: false,
		Address: ":9090",
		Path:    "/metrics",
	}
}

// Server represents the metrics HTTP server
type Server struct {
	config *Config
	server *http.Server
}

// NewServer creates a new metrics server
func NewServer(cfg *Config) *Server {
	if cfg == nil {
		cfg = DefaultConfig()
	}

	mux := http.NewServeMux()
	mux.Handle(cfg.Path, promhttp.Handler())

	return &Server{
		config: cfg,
		server: &http.Server{
			Addr:              cfg.Address,
			Handler:           mux,
			ReadHeaderTimeout: 10 * time.Second,
		},
	}
}

// Start starts the metrics server
func (s *Server) Start() error {
	if !s.config.Enabled {
		return nil
	}
	return s.server.ListenAndServe()
}

// Stop stops the metrics server
func (s *Server) Stop() error {
	if !s.config.Enabled {
		return nil
	}
	return s.server.Close()
}
