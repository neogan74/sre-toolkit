package mock

import (
	"fmt"
	"math/rand"
	"net/http"
	"time"

	"github.com/neogan/sre-toolkit/pkg/logging"
)

// ServerConfig holds configuration for the mock server
type ServerConfig struct {
	Port      int
	ErrorRate int           // Percentage 0-100
	Latency   time.Duration // Sleep duration
	Jitter    time.Duration // Latency variation (+/-)
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
