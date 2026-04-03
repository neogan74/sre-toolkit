package http

import (
	"context"
	"errors"
	"io"
	"net/http"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"
)

type capturedRequest struct {
	Method        string
	Body          string
	Authorization string
}

type stubTransport struct {
	statusCode int
	err        error
	delay      time.Duration

	count    atomic.Int64
	mu       sync.Mutex
	requests []capturedRequest
}

func (s *stubTransport) RoundTrip(req *http.Request) (*http.Response, error) {
	if s.delay > 0 {
		select {
		case <-req.Context().Done():
			return nil, req.Context().Err()
		case <-time.After(s.delay):
		}
	}

	s.count.Add(1)

	bodyBytes, err := io.ReadAll(req.Body)
	if err != nil {
		return nil, err
	}
	req.Body.Close()

	s.mu.Lock()
	s.requests = append(s.requests, capturedRequest{
		Method:        req.Method,
		Body:          string(bodyBytes),
		Authorization: req.Header.Get("Authorization"),
	})
	s.mu.Unlock()

	if s.err != nil {
		return nil, s.err
	}

	return &http.Response{
		StatusCode: s.statusCode,
		Body:       io.NopCloser(strings.NewReader("ok")),
		Header:     make(http.Header),
		Request:    req,
	}, nil
}

func (s *stubTransport) requestCount() int64 {
	return s.count.Load()
}

func (s *stubTransport) firstRequest(t *testing.T) capturedRequest {
	t.Helper()

	s.mu.Lock()
	defer s.mu.Unlock()

	if len(s.requests) == 0 {
		t.Fatal("expected at least one captured request")
	}

	return s.requests[0]
}

func newTestPool(cfg PoolConfig, transport http.RoundTripper) *Pool {
	pool := NewPool(cfg)
	pool.client = &http.Client{
		Timeout:   2 * time.Second,
		Transport: transport,
	}

	return pool
}

func TestPoolRunHonorsRequestLimit(t *testing.T) {
	transport := &stubTransport{statusCode: http.StatusOK}
	pool := newTestPool(PoolConfig{
		TargetURL:   "https://example.com",
		Concurrency: 5,
		Duration:    100 * time.Millisecond,
		Requests:    20,
	}, transport)

	if err := pool.Run(); err != nil {
		t.Fatalf("Pool.Run() failed: %v", err)
	}

	if got := transport.requestCount(); got != 20 {
		t.Fatalf("expected 20 requests, got %d", got)
	}
}

func TestPoolRunStopsAroundDuration(t *testing.T) {
	transport := &stubTransport{
		statusCode: http.StatusOK,
		delay:      10 * time.Millisecond,
	}
	pool := newTestPool(PoolConfig{
		TargetURL:   "https://example.com",
		Concurrency: 2,
		Duration:    50 * time.Millisecond,
	}, transport)

	start := time.Now()
	if err := pool.Run(); err != nil {
		t.Fatalf("Pool.Run() failed: %v", err)
	}
	elapsed := time.Since(start)

	if elapsed < 50*time.Millisecond || elapsed > 250*time.Millisecond {
		t.Fatalf("expected runtime around 50ms, got %v", elapsed)
	}
}

func TestPoolRunCapturesRequestErrors(t *testing.T) {
	transport := &stubTransport{err: errors.New("boom")}
	pool := newTestPool(PoolConfig{
		TargetURL:   "https://example.com",
		Concurrency: 1,
		Duration:    50 * time.Millisecond,
		Requests:    1,
	}, transport)

	if err := pool.Run(); err != nil {
		t.Fatalf("Pool.Run() should not fail even if requests fail: %v", err)
	}

	if got := transport.requestCount(); got != 1 {
		t.Fatalf("expected 1 request attempt, got %d", got)
	}
}

func TestPoolRunUsesMethodAndBody(t *testing.T) {
	transport := &stubTransport{statusCode: http.StatusOK}
	pool := newTestPool(PoolConfig{
		TargetURL:   "https://example.com",
		Method:      http.MethodPost,
		Body:        "hello world",
		Concurrency: 1,
		Duration:    time.Second,
		Requests:    1,
	}, transport)

	if err := pool.Run(); err != nil {
		t.Fatalf("Pool.Run() failed: %v", err)
	}

	req := transport.firstRequest(t)
	if req.Method != http.MethodPost {
		t.Fatalf("expected method POST, got %s", req.Method)
	}
	if req.Body != "hello world" {
		t.Fatalf("expected body %q, got %q", "hello world", req.Body)
	}
}

func TestPoolNewRequestSetsBearerToken(t *testing.T) {
	pool := NewPool(PoolConfig{
		TargetURL:   "https://example.com",
		BearerToken: "token-123",
	})

	req, err := pool.newRequest(context.Background())
	if err != nil {
		t.Fatalf("newRequest() failed: %v", err)
	}

	if got := req.Header.Get("Authorization"); got != "Bearer token-123" {
		t.Fatalf("expected bearer authorization header, got %q", got)
	}
}

func TestPoolNewRequestSetsBasicAuth(t *testing.T) {
	pool := NewPool(PoolConfig{
		TargetURL:     "https://example.com",
		BasicUsername: "demo",
		BasicPassword: "secret",
	})

	req, err := pool.newRequest(context.Background())
	if err != nil {
		t.Fatalf("newRequest() failed: %v", err)
	}

	username, password, ok := req.BasicAuth()
	if !ok {
		t.Fatal("expected basic auth header to be set")
	}
	if username != "demo" || password != "secret" {
		t.Fatalf("unexpected basic auth credentials: %q / %q", username, password)
	}
}

func TestPoolConfigValidateRejectsConflictingAuthModes(t *testing.T) {
	err := (PoolConfig{
		TargetURL:     "https://example.com",
		BearerToken:   "token-123",
		BasicUsername: "demo",
	}).Validate()
	if err == nil {
		t.Fatal("expected validation error for conflicting auth modes")
	}
}

func TestPoolConfigValidateRequiresBasicUsername(t *testing.T) {
	err := (PoolConfig{
		TargetURL:     "https://example.com",
		BasicPassword: "secret",
	}).Validate()
	if err == nil {
		t.Fatal("expected validation error when basic password is set without username")
	}
}

func TestPoolConfigValidate_Valid(t *testing.T) {
	cases := []struct {
		name string
		cfg  PoolConfig
	}{
		{"no auth", PoolConfig{TargetURL: "https://example.com"}},
		{"bearer only", PoolConfig{TargetURL: "https://example.com", BearerToken: "tok"}},
		{"basic only", PoolConfig{TargetURL: "https://example.com", BasicUsername: "u", BasicPassword: "p"}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if err := tc.cfg.Validate(); err != nil {
				t.Fatalf("Validate() unexpected error: %v", err)
			}
		})
	}
}

func TestPoolNewRequest_DefaultMethodIsGET(t *testing.T) {
	pool := NewPool(PoolConfig{TargetURL: "https://example.com"})
	req, err := pool.newRequest(context.Background())
	if err != nil {
		t.Fatalf("newRequest() failed: %v", err)
	}
	if req.Method != http.MethodGet {
		t.Fatalf("expected default method GET, got %s", req.Method)
	}
}

func TestPoolNewRequest_InvalidURLReturnsError(t *testing.T) {
	pool := NewPool(PoolConfig{TargetURL: "://bad url"})
	_, err := pool.newRequest(context.Background())
	if err == nil {
		t.Fatal("expected error for invalid URL")
	}
}
