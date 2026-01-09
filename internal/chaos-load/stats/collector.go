// Package stats provides statistics collection and reporting for load tests.
package stats

import (
	"fmt"
	"io"
	"os"
	"sort"
	"sync"
	"time"
)

// Result represents the result of a single request
type Result struct {
	StatusCode int
	Duration   time.Duration
	Error      error
}

// Collector aggregates results from multiple workers
type Collector struct {
	results []Result
	mu      sync.Mutex
	start   time.Time
}

// NewCollector creates a new stats collector
func NewCollector() *Collector {
	return &Collector{
		results: make([]Result, 0, 1000),
		start:   time.Now(),
	}
}

// Add records a single result
func (c *Collector) Add(r Result) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.results = append(c.results, r)
}

// Report prints a summary report to stdout
func (c *Collector) Report() {
	c.FprintReport(os.Stdout)
}

// FprintReport generates a summary report to the given writer
func (c *Collector) FprintReport(w io.Writer) {
	c.mu.Lock()
	defer c.mu.Unlock()

	total := len(c.results)
	if total == 0 {
		fmt.Fprintln(w, "No requests made")
		return
	}

	var totalDuration time.Duration
	var errors int
	statusCodes := make(map[int]int)
	durations := make([]time.Duration, 0, total)

	for _, r := range c.results {
		if r.Error != nil {
			errors++
		} else {
			statusCodes[r.StatusCode]++
			durations = append(durations, r.Duration)
			totalDuration += r.Duration
		}
	}

	elapsed := time.Since(c.start)
	rps := float64(total) / elapsed.Seconds()

	fmt.Fprintf(w, "\n=== Load Test Results ===\n")
	fmt.Fprintf(w, "Total Requests: %d\n", total)
	fmt.Fprintf(w, "Total Duration: %v\n", elapsed)
	fmt.Fprintf(w, "Requests/sec:   %.2f\n", rps)
	fmt.Fprintf(w, "Errors:         %d\n", errors)

	if len(durations) > 0 {
		sort.Slice(durations, func(i, j int) bool {
			return durations[i] < durations[j]
		})

		count := len(durations)
		fmt.Fprintf(w, "\nLatency:\n")
		fmt.Fprintf(w, "  p50: %v\n", durations[count/2])
		fmt.Fprintf(w, "  p95: %v\n", durations[int(float64(count)*0.95)])
		fmt.Fprintf(w, "  p99: %v\n", durations[int(float64(count)*0.99)])
		fmt.Fprintf(w, "  Max: %v\n", durations[count-1])
	}

	if len(statusCodes) > 0 {
		fmt.Fprintf(w, "\nStatus Codes:\n")
		// Sort status codes for deterministic output
		keys := make([]int, 0, len(statusCodes))
		for k := range statusCodes {
			keys = append(keys, k)
		}
		sort.Ints(keys)
		for _, code := range keys {
			fmt.Fprintf(w, "  [%d]: %d\n", code, statusCodes[code])
		}
	}
}
