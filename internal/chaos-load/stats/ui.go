package stats

import (
	"context"
	"fmt"
	"sort"
	"time"
)

const (
	clearScreen = "\033[H\033[2J"
	cursorTop   = "\033[H"
)

// UI represents the real-time terminal dashboard.
type UI struct {
	collector *Collector
	interval  time.Duration
}

// NewUI creates a new UI component bound to a collector.
func NewUI(c *Collector, refreshRate time.Duration) *UI {
	return &UI{
		collector: c,
		interval:  refreshRate,
	}
}

// Run starts the UI refresh loop until the context is canceled.
func (u *UI) Run(ctx context.Context) {
	ticker := time.NewTicker(u.interval)
	defer ticker.Stop()

	// Initial clear
	fmt.Print(clearScreen)

	for {
		select {
		case <-ctx.Done():
			// Do one final render before exiting
			u.render()
			fmt.Println("\n\nLoad test complete.")
			return
		case <-ticker.C:
			u.render()
		}
	}
}

func (u *UI) render() {
	snapshot := u.collector.Snapshot()

	// Move cursor to top-left to overwrite
	fmt.Print(cursorTop)

	fmt.Println("=== Chaos-Load Real-Time Dashboard ===")
	fmt.Printf("Elapsed:      %v\n", snapshot.Elapsed.Round(time.Second))
	fmt.Printf("Requests:     %d\n", snapshot.TotalRequests)
	fmt.Printf("Errors:       %d\n", snapshot.Errors)
	fmt.Printf("RPS:          %.2f req/s\n", snapshot.RPS)

	fmt.Println("\nStatus Codes:")
	if len(snapshot.StatusCodes) == 0 {
		fmt.Println("  None yet")
	} else {
		// sort keys
		keys := make([]int, 0, len(snapshot.StatusCodes))
		for k := range snapshot.StatusCodes {
			keys = append(keys, k)
		}
		sort.Ints(keys)
		for _, k := range keys {
			fmt.Printf("  [%d]: %d\n", k, snapshot.StatusCodes[k])
		}
	}
	// Print a few empty lines to clear out any residual lines if the list shrinks (unlikely)
	fmt.Println("\n                                    ")
	fmt.Println("                                    ")
}
