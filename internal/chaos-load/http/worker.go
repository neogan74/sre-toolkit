// Package http provides HTTP load testing functionality for chaos engineering.
package http

import (
	"context"
	"net/http"
	"strings"
	"sync"
	"time"

	"github.com/neogan/sre-toolkit/internal/chaos-load/stats"
	"github.com/neogan/sre-toolkit/pkg/logging"
)

// PoolConfig holds configuration for the worker pool
type PoolConfig struct {
	TargetURL   string
	Method      string
	Body        string
	Concurrency int
	Duration    time.Duration
	Requests    int // Optional limit on total requests
}

// Pool manages a pool of HTTP workers
type Pool struct {
	config    PoolConfig
	client    *http.Client
	collector *stats.Collector
}

// NewPool creates a new worker pool
func NewPool(cfg PoolConfig) *Pool {
	return &Pool{
		config: cfg,
		client: &http.Client{
			Timeout: 10 * time.Second,
			Transport: &http.Transport{
				MaxIdleConns:        cfg.Concurrency,
				MaxIdleConnsPerHost: cfg.Concurrency,
				IdleConnTimeout:     90 * time.Second,
				DisableKeepAlives:   false,
			},
		},
		collector: stats.NewCollector(),
	}
}

// Run starts the load test
func (p *Pool) Run() error {
	logger := logging.GetLogger()
	ctx, cancel := context.WithTimeout(context.Background(), p.config.Duration)
	defer cancel()

	var wg sync.WaitGroup
	requestsCh := make(chan struct{}, p.config.Concurrency)

	// If request limit is set, feed the channel
	if p.config.Requests > 0 {
		go func() {
			for i := 0; i < p.config.Requests; i++ {
				select {
				case <-ctx.Done():
					return
				case requestsCh <- struct{}{}:
				}
			}
			close(requestsCh)
		}()
	} else {
		// Infinite mode
		go func() {
			for {
				select {
				case <-ctx.Done():
					close(requestsCh)
					return
				case requestsCh <- struct{}{}:
				}
			}
		}()
	}

	logger.Info().
		Int("concurrency", p.config.Concurrency).
		Dur("duration", p.config.Duration).
		Msg("Starting workers")

	for i := 0; i < p.config.Concurrency; i++ {
		wg.Add(1)
		go func(id int) {
			defer wg.Done()
			p.worker(ctx, id, requestsCh)
		}(i)
	}

	// Wait for completion
	wg.Wait()
	logger.Info().Msg("Load test completed")

	// Report results
	p.collector.Report()

	return nil
}

func (p *Pool) worker(ctx context.Context, _ /* id */ int, requests <-chan struct{}) {
	for range requests {
		// check context cancellation
		select {
		case <-ctx.Done():
			return
		default:
		}

		start := time.Now()

		method := p.config.Method
		if method == "" {
			method = http.MethodGet
		}

		var body *strings.Reader
		if p.config.Body != "" {
			body = strings.NewReader(p.config.Body)
		}

		// Re-create request for each iteration since body is read
		// If body is needed we might need to reset reader or create new one.
		// strings.NewReader creation is cheap.
		var req *http.Request
		var err error

		if body != nil {
			req, err = http.NewRequestWithContext(ctx, method, p.config.TargetURL, body)
		} else {
			req, err = http.NewRequestWithContext(ctx, method, p.config.TargetURL, http.NoBody)
		}
		var resp *http.Response
		if err == nil {
			resp, err = p.client.Do(req)
		}
		duration := time.Since(start)

		result := stats.Result{
			Duration: duration,
			Error:    err,
		}

		if err == nil {
			result.StatusCode = resp.StatusCode
			resp.Body.Close()
		}

		p.collector.Add(result)
	}
}
