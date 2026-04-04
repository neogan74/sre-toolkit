package mock

import (
	"bufio"
	"context"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

// responseWriterAdapter wraps httptest.ResponseRecorder and adds Hijack support for testing connection drops.
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
		Port:    0, // not used in httptest
		Latency: latency,
		Jitter:  jitter,
	}

	s := NewServer(cfg)

	// Create a test handler that uses the server logic
	// We can't easily test s.Run() as it blocks, so we test handleRequest directly
	handler := http.HandlerFunc(s.handleRequest)
	ts := httptest.NewServer(handler)
	defer ts.Close()

	client := ts.Client()

	for i := 0; i < 20; i++ {
		start := time.Now()
		resp, err := client.Get(ts.URL)
		assert.NoError(t, err)
		duration := time.Since(start)
		resp.Body.Close()

		// Allow some overhead for test execution (e.g. +20ms buffer)
		// Min duration should be close to latency - jitter
		// Max duration should be close to latency + jitter (+ buffer)

		// Allow significant overhead for test execution in CI/loaded envs
		// Min duration should be close to latency - jitter
		minDuration := latency - jitter
		maxDuration := latency + jitter + (500 * time.Millisecond) // substantial buffer

		// Note: time.Sleep is not guaranteed to be precise.
		// Testing this strictly is flaky. We check if it's "around" the expected values.
		assert.GreaterOrEqual(t, duration, minDuration, "Duration too short")
		assert.LessOrEqual(t, duration, maxDuration, "Duration too long")

		// If system is slow, this might fail, but it's a good sanity check
		t.Logf("Request took %v (expected %v +/- %v)", duration, latency, jitter)
	}
}

func TestServer_ErrorRate(t *testing.T) {
	cfg := ServerConfig{
		ErrorRate: 100, // 100% errors
	}

	s := NewServer(cfg)
	handler := http.HandlerFunc(s.handleRequest)
	ts := httptest.NewServer(handler)
	defer ts.Close()

	resp, err := ts.Client().Get(ts.URL)
	assert.NoError(t, err)
	assert.Equal(t, http.StatusInternalServerError, resp.StatusCode)
	resp.Body.Close()
}

func TestServer_ConnectionFailure_DropsConnection(t *testing.T) {
	s := NewServer(ServerConfig{ConnectionFailureRate: 100})

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
		req := httptest.NewRequest(http.MethodGet, "/", nil)
		s.handleRequest(writer, req)
	}()

	// The server closes the connection — client should get EOF
	buf := make([]byte, 1)
	_, err := clientConn.Read(buf)
	assert.ErrorIs(t, err, io.EOF)
	<-done
}

func TestServer_ConnectionFailure_FallsBackWhenHijackNotSupported(t *testing.T) {
	s := NewServer(ServerConfig{ConnectionFailureRate: 100})

	recorder := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	s.handleRequest(recorder, req)

	assert.Equal(t, http.StatusServiceUnavailable, recorder.Code)
	assert.Contains(t, recorder.Body.String(), "Connection failure simulation requires hijack support")
}

func TestServer_ConnectionFailure_ZeroRateDoesNotDrop(t *testing.T) {
	s := NewServer(ServerConfig{ConnectionFailureRate: 0})
	recorder := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	s.handleRequest(recorder, req)
	assert.Equal(t, http.StatusOK, recorder.Code)
}

func TestServer_ZeroErrorRate(t *testing.T) {
	s := NewServer(ServerConfig{ErrorRate: 0})
	handler := http.HandlerFunc(s.handleRequest)
	ts := httptest.NewServer(handler)
	defer ts.Close()

	resp, err := ts.Client().Get(ts.URL)
	assert.NoError(t, err)
	assert.Equal(t, http.StatusOK, resp.StatusCode)
	resp.Body.Close()
}

func TestServer_LatencyNoJitter(t *testing.T) {
	latency := 50 * time.Millisecond
	s := NewServer(ServerConfig{Latency: latency})
	handler := http.HandlerFunc(s.handleRequest)
	ts := httptest.NewServer(handler)
	defer ts.Close()

	start := time.Now()
	resp, err := ts.Client().Get(ts.URL)
	elapsed := time.Since(start)
	assert.NoError(t, err)
	resp.Body.Close()

	assert.GreaterOrEqual(t, elapsed, latency, "should take at least the configured latency")
	assert.Less(t, elapsed, latency+500*time.Millisecond, "should not take excessively long")
}

func TestServer_Run_StartsAndShutdown(t *testing.T) {
	// Find a free port
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	assert.NoError(t, err)
	port := ln.Addr().(*net.TCPAddr).Port
	ln.Close()

	s := NewServer(ServerConfig{Port: port})

	errCh := make(chan error, 1)
	go func() {
		errCh <- s.Run()
	}()

	// Wait for the server to start
	addr := fmt.Sprintf("http://127.0.0.1:%d/", port)
	var resp *http.Response
	for i := 0; i < 20; i++ {
		resp, err = http.Get(addr) //nolint:noctx
		if err == nil {
			resp.Body.Close()
			break
		}
		time.Sleep(10 * time.Millisecond)
	}
	assert.NoError(t, err, "server should be reachable")

	// Shutdown gracefully
	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()
	assert.NoError(t, s.server.Shutdown(ctx))

	// Run() should return http.ErrServerClosed (treated as normal exit)
	select {
	case runErr := <-errCh:
		assert.ErrorIs(t, runErr, http.ErrServerClosed)
	case <-time.After(2 * time.Second):
		t.Fatal("server did not stop within timeout")
	}
}
