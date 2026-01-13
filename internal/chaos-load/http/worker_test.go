package http

import (
	"net/http"
	"net/http/httptest"
	"sync/atomic"
	"testing"
	"time"
)

func TestPool_Run(t *testing.T) {
	var requestCount int64
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		atomic.AddInt64(&requestCount, 1)
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	cfg := PoolConfig{
		TargetURL:   server.URL,
		Concurrency: 5,
		Duration:    100 * time.Millisecond,
		Requests:    20,
	}

	pool := NewPool(cfg)
	err := pool.Run()
	if err != nil {
		t.Fatalf("Pool.Run() failed: %v", err)
	}

	count := atomic.LoadInt64(&requestCount)
	if count != 20 {
		t.Errorf("expected 20 requests, but got %d", count)
	}
}

func TestPool_Duration(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		time.Sleep(10 * time.Millisecond)
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	cfg := PoolConfig{
		TargetURL:   server.URL,
		Concurrency: 2,
		Duration:    50 * time.Millisecond,
		Requests:    0, // Infinite
	}

	start := time.Now()
	pool := NewPool(cfg)
	err := pool.Run()
	if err != nil {
		t.Fatalf("Pool.Run() failed: %v", err)
	}
	elapsed := time.Since(start)

	// Should be around 50ms + some overhead
	if elapsed < 50*time.Millisecond || elapsed > 200*time.Millisecond {
		t.Errorf("expected duration to be around 50ms, but got %v", elapsed)
	}
}

func TestPool_Error(t *testing.T) {
	// No server running at this URL should cause errors
	cfg := PoolConfig{
		TargetURL:   "http://localhost:1", // Invalid port
		Concurrency: 1,
		Duration:    50 * time.Millisecond,
		Requests:    1,
	}

	pool := NewPool(cfg)
	err := pool.Run()
	if err != nil {
		t.Fatalf("Pool.Run() should not fail even if requests fail: %v", err)
	}

	// Internal collector should have captured the error
	// results are not exported, but we can check if it reported errors if we capture stdout
	// but better to check the results slice if we could.
	// Since we are checking if it works without panic, this is already a good start.
}

func TestPool_MethodsAndBody(t *testing.T) {
	var receivedMethod string
	var receivedBody string

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		receivedMethod = r.Method
		// read body
		buf := make([]byte, 1024)
		n, _ := r.Body.Read(buf)
		receivedBody = string(buf[:n])

		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	cfg := PoolConfig{
		TargetURL:   server.URL,
		Method:      "POST",
		Body:        "hello world",
		Concurrency: 1,
		Duration:    time.Second,
		Requests:    1,
	}

	pool := NewPool(cfg)
	if err := pool.Run(); err != nil {
		t.Fatalf("Pool.Run() failed: %v", err)
	}

	if receivedMethod != "POST" {
		t.Errorf("expected method POST, got %s", receivedMethod)
	}
	if receivedBody != "hello world" {
		t.Errorf("expected body 'hello world', got '%s'", receivedBody)
	}
}
