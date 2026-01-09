package stats

import (
	"bytes"
	"errors"
	"strings"
	"testing"
	"time"
)

func TestCollector_AddAndReport(t *testing.T) {
	c := NewCollector()
	c.start = time.Now().Add(-1 * time.Second) // Fake 1 second elapsed

	c.Add(Result{StatusCode: 200, Duration: 10 * time.Millisecond})
	c.Add(Result{StatusCode: 200, Duration: 20 * time.Millisecond})
	c.Add(Result{StatusCode: 404, Duration: 5 * time.Millisecond})
	c.Add(Result{StatusCode: 0, Error: errors.New("timeout"), Duration: 0})

	var buf bytes.Buffer
	c.FprintReport(&buf)
	output := buf.String()

	expectedSubstrings := []string{
		"Total Requests: 4",
		"Errors:         1",
		"[200]: 2",
		"[404]: 1",
		"p50:",
		"p95:",
	}

	for _, s := range expectedSubstrings {
		if !strings.Contains(output, s) {
			t.Errorf("expected output to contain %q, but it didn't. Output:\n%s", s, output)
		}
	}
}

func TestCollector_Empty(t *testing.T) {
	c := NewCollector()
	var buf bytes.Buffer
	c.FprintReport(&buf)
	output := buf.String()

	if !strings.Contains(output, "No requests made") {
		t.Errorf("expected output to contain %q for empty collector", "No requests made")
	}
}

func TestCollector_Percentiles(t *testing.T) {
	c := NewCollector()
	// Add 100 results with increasing duration
	for i := 1; i <= 100; i++ {
		c.Add(Result{StatusCode: 200, Duration: time.Duration(i) * time.Millisecond})
	}

	var buf bytes.Buffer
	c.FprintReport(&buf)
	output := buf.String()

	// 100 requests. p50 should be 51ms (index 50), p95 should be 96ms (index 95), p99 should be 100ms (index 99)
	// indices in slice: 0-99
	// durations[50] = 51ms
	// durations[95] = 96ms
	// durations[99] = 100ms

	expectedPercentiles := []string{
		"p50: 51ms",
		"p95: 96ms",
		"p99: 100ms",
	}

	for _, s := range expectedPercentiles {
		if !strings.Contains(output, s) {
			t.Errorf("expected output to contain %q, but it didn't. Output:\n%s", s, output)
		}
	}
}
