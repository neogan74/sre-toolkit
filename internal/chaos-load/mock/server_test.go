package mock

import (
	"bufio"
	"io"
	"net"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

type responseWriterAdapter struct {
	http.ResponseWriter
	hijack func() (net.Conn, *bufio.ReadWriter, error)
}

func (r *responseWriterAdapter) Hijack() (net.Conn, *bufio.ReadWriter, error) {
	return r.hijack()
}

func TestServer_LatencyWithJitter(t *testing.T) {
	latency := 100 * time.Millisecond
	jitter := 50 * time.Millisecond

	cfg := ServerConfig{
		Latency: latency,
		Jitter:  jitter,
	}

	s := NewServer(cfg)

	for i := 0; i < 20; i++ {
		recorder := httptest.NewRecorder()
		req := httptest.NewRequest(http.MethodGet, "http://example.com", nil)

		start := time.Now()
		s.handleRequest(recorder, req)
		duration := time.Since(start)

		minDuration := latency - jitter
		maxDuration := latency + jitter + (500 * time.Millisecond)

		assert.GreaterOrEqual(t, duration, minDuration, "Duration too short")
		assert.LessOrEqual(t, duration, maxDuration, "Duration too long")
		assert.Equal(t, http.StatusOK, recorder.Code)
	}
}

func TestServer_ErrorRate(t *testing.T) {
	cfg := ServerConfig{
		ErrorRate: 100, // 100% errors
	}

	s := NewServer(cfg)
	recorder := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "http://example.com", nil)

	s.handleRequest(recorder, req)

	assert.Equal(t, http.StatusInternalServerError, recorder.Code)
	assert.Contains(t, recorder.Body.String(), "Internal Server Error")
}

func TestServer_ConnectionFailureRate(t *testing.T) {
	cfg := ServerConfig{
		ConnectionFailureRate: 100,
	}

	s := NewServer(cfg)
	serverConn, clientConn := net.Pipe()
	defer clientConn.Close()

	writer := &responseWriterAdapter{
		ResponseWriter: httptest.NewRecorder(),
		hijack: func() (net.Conn, *bufio.ReadWriter, error) {
			return serverConn, bufio.NewReadWriter(bufio.NewReader(serverConn), bufio.NewWriter(serverConn)), nil
		},
	}

	done := make(chan struct{})
	go func() {
		defer close(done)
		req := httptest.NewRequest(http.MethodGet, "http://example.com", nil)
		s.handleRequest(writer, req)
	}()

	buf := make([]byte, 1)
	_, err := clientConn.Read(buf)
	assert.ErrorIs(t, err, io.EOF)
	<-done
}

func TestServer_ConnectionFailureRequiresHijackSupport(t *testing.T) {
	cfg := ServerConfig{
		ConnectionFailureRate: 100,
	}

	s := NewServer(cfg)
	recorder := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "http://example.com", nil)

	s.handleRequest(recorder, req)

	assert.Equal(t, http.StatusServiceUnavailable, recorder.Code)
	assert.Contains(t, recorder.Body.String(), "Connection failure simulation requires hijack support")
}

func TestServer_TimeoutSimulation(t *testing.T) {
	cfg := ServerConfig{
		TimeoutRate:     100,
		TimeoutDuration: 40 * time.Millisecond,
	}

	s := NewServer(cfg)
	recorder := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "http://example.com", nil)

	start := time.Now()
	s.handleRequest(recorder, req)
	duration := time.Since(start)

	assert.GreaterOrEqual(t, duration, cfg.TimeoutDuration)
	assert.Equal(t, http.StatusGatewayTimeout, recorder.Code)
	assert.Contains(t, recorder.Body.String(), "Gateway Timeout")
}

func TestServerConfigValidateRequiresTimeoutDuration(t *testing.T) {
	err := (ServerConfig{
		TimeoutRate: 10,
	}).Validate()

	assert.EqualError(t, err, "timeout simulation requires --timeout-duration > 0")
}

func TestServerConfigValidateRejectsInvalidRates(t *testing.T) {
	err := (ServerConfig{
		TimeoutRate: 101,
	}).Validate()

	assert.EqualError(t, err, "timeout-rate must be between 0 and 100")
}
