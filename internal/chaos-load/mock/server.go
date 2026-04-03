package mock

import (
	"errors"
	"fmt"
	"math/rand"
	"net/http"
	"time"

	"github.com/neogan/sre-toolkit/pkg/logging"
)

// ServerConfig holds configuration for the mock server
type ServerConfig struct {
	Port                  int
	ErrorRate             int           // Percentage 0-100
	ConnectionFailureRate int           // Percentage 0-100
	TimeoutRate           int           // Percentage 0-100
	TimeoutDuration       time.Duration // How long to delay before returning 504
	Latency               time.Duration // Sleep duration
	Jitter                time.Duration // Latency variation (+/-)
}

// Server represents a chaos mock server
type Server struct {
	config ServerConfig
	server *http.Server
}

// NewServer creates a new mock server
func NewServer(cfg ServerConfig) *Server {
	return &Server{
		config: cfg,
	}
}

// Validate ensures the mock server configuration is internally consistent.
func (c ServerConfig) Validate() error {
	if err := validatePercentage("error-rate", c.ErrorRate); err != nil {
		return err
	}
	if err := validatePercentage("connection-failure-rate", c.ConnectionFailureRate); err != nil {
		return err
	}
	if err := validatePercentage("timeout-rate", c.TimeoutRate); err != nil {
		return err
	}
	if c.TimeoutRate > 0 && c.TimeoutDuration <= 0 {
		return errors.New("timeout simulation requires --timeout-duration > 0")
	}

	return nil
}

// Run starts the mock server
func (s *Server) Run() error {
	logger := logging.GetLogger()

	mux := http.NewServeMux()
	mux.HandleFunc("/", s.handleRequest)

	addr := fmt.Sprintf(":%d", s.config.Port)
	s.server = &http.Server{
		Addr:    addr,
		Handler: mux,
	}

	logger.Info().
		Int("port", s.config.Port).
		Int("error_rate", s.config.ErrorRate).
		Int("connection_failure_rate", s.config.ConnectionFailureRate).
		Int("timeout_rate", s.config.TimeoutRate).
		Dur("timeout_duration", s.config.TimeoutDuration).
		Dur("latency", s.config.Latency).
		Dur("jitter", s.config.Jitter).
		Msg("Starting chaos mock server")

	return s.server.ListenAndServe()
}

func (s *Server) handleRequest(w http.ResponseWriter, r *http.Request) {
	logger := logging.GetLogger()
	start := time.Now()

	// Simulate latency with jitter
	if s.config.Latency > 0 {
		delay := s.config.Latency
		if s.config.Jitter > 0 {
			jitterNanos := s.config.Jitter.Nanoseconds()
			// Random offset in range [-jitter, +jitter]
			offsetNanos := rand.Int63n(2*jitterNanos+1) - jitterNanos
			delay += time.Duration(offsetNanos)
			if delay < 0 {
				delay = 0
			}
		}
		time.Sleep(delay)
	}

	// Simulate error
	if s.shouldFailConnection() {
		if err := closeConnection(w); err != nil {
			logger.Warn().Err(err).Msg("Failed to drop client connection")
			http.Error(w, "Connection failure simulation requires hijack support", http.StatusServiceUnavailable)
			return
		}

		logger.Info().
			Str("method", r.Method).
			Str("path", r.URL.Path).
			Dur("duration", time.Since(start)).
			Msg("Connection dropped")
		return
	}

	if s.shouldTimeout() {
		time.Sleep(s.config.TimeoutDuration)
		http.Error(w, "Gateway Timeout", http.StatusGatewayTimeout)

		logger.Info().
			Str("method", r.Method).
			Str("path", r.URL.Path).
			Int("status", http.StatusGatewayTimeout).
			Dur("duration", time.Since(start)).
			Msg("Timeout simulated")
		return
	}

	statusCode := http.StatusOK
	if s.config.ErrorRate > 0 {
		// rand.Intn(100) returns 0-99
		// If ErrorRate is 10, we want 10% chance. 0-9 < 10.
		if rand.Intn(100) < s.config.ErrorRate {
			statusCode = http.StatusInternalServerError
		}
	}

	w.WriteHeader(statusCode)
	if statusCode == http.StatusOK {
		w.Write([]byte("OK"))
	} else {
		w.Write([]byte("Internal Server Error"))
	}

	logger.Info().
		Str("method", r.Method).
		Str("path", r.URL.Path).
		Int("status", statusCode).
		Dur("duration", time.Since(start)).
		Msg("Request processed")
}

func (s *Server) shouldFailConnection() bool {
	return s.config.ConnectionFailureRate > 0 && rand.Intn(100) < s.config.ConnectionFailureRate
}

func (s *Server) shouldTimeout() bool {
	return s.config.TimeoutRate > 0 && rand.Intn(100) < s.config.TimeoutRate
}

func closeConnection(w http.ResponseWriter) error {
	hijacker, ok := w.(http.Hijacker)
	if !ok {
		return fmt.Errorf("response writer does not support hijacking")
	}

	conn, _, err := hijacker.Hijack()
	if err != nil {
		return err
	}

	return conn.Close()
}

func validatePercentage(flag string, value int) error {
	if value < 0 || value > 100 {
		return fmt.Errorf("%s must be between 0 and 100", flag)
	}

	return nil
}
