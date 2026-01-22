package mock

import (
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

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
