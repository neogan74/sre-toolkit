package analyzer

import (
	"context"
	"fmt"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

const traceLog = `{"time":"2026-01-01T10:00:00Z","level":"info","msg":"start","trace_id":"t1"}
{"time":"2026-01-01T10:00:02Z","level":"error","msg":"db down","trace_id":"t1"}
{"time":"2026-01-01T10:00:03Z","level":"error","msg":"retry failed","trace_id":"t1"}
{"time":"2026-01-01T10:00:01Z","level":"error","msg":"bad input","trace_id":"t2"}
{"time":"2026-01-01T10:00:01Z","level":"info","msg":"fine","trace_id":"t3"}
{"time":"2026-01-01T10:00:04Z","level":"info","msg":"no id"}
`

func TestAnalyze_TraceGroups(t *testing.T) {
	stats, err := Analyze(context.Background(), strings.NewReader(traceLog), DefaultConfig())
	require.NoError(t, err)

	assert.Equal(t, 3, stats.TracesTotal)
	require.Len(t, stats.Traces, 2, "t3 has no errors")
	first := stats.Traces[0]
	assert.Equal(t, "t1", first.ID)
	assert.Equal(t, 3, first.Entries)
	assert.Equal(t, 2, first.Errors)
	assert.Equal(t, 3*time.Second, first.Duration)
	assert.Equal(t, "db down", first.FirstError)
	assert.Equal(t, "t2", stats.Traces[1].ID)
}

func TestAnalyze_TraceCustomKey(t *testing.T) {
	cfg := DefaultConfig()
	cfg.TraceKeys = []string{"order_id"}
	in := `{"level":"error","msg":"x","order_id":"o1","trace_id":"t1"}` + "\n"
	stats, err := Analyze(context.Background(), strings.NewReader(in), cfg)
	require.NoError(t, err)
	require.Len(t, stats.Traces, 1)
	assert.Equal(t, "o1", stats.Traces[0].ID)
}

func TestAnalyze_TraceLimit(t *testing.T) {
	var b strings.Builder
	for i := 0; i < 5; i++ {
		fmt.Fprintf(&b, `{"level":"error","msg":"x","trace_id":"t%d"}`+"\n", i)
	}
	cfg := DefaultConfig()
	cfg.MaxTraces = 2
	stats, err := Analyze(context.Background(), strings.NewReader(b.String()), cfg)
	require.NoError(t, err)
	assert.Equal(t, 2, stats.TracesTotal)
	assert.Equal(t, 3, stats.TracesLost)
}

func TestTraceTracker_ErrorsReplaceFullSamples(t *testing.T) {
	var b strings.Builder
	for i := 0; i < maxTraceSamples+5; i++ {
		b.WriteString(`{"level":"info","msg":"i","trace_id":"t"}` + "\n")
	}
	b.WriteString(`{"level":"error","msg":"late failure","trace_id":"t"}` + "\n")
	stats, err := Analyze(context.Background(), strings.NewReader(b.String()), DefaultConfig())
	require.NoError(t, err)
	require.Len(t, stats.Traces, 1)
	g := stats.Traces[0]
	assert.Len(t, g.Samples, maxTraceSamples)
	assert.Equal(t, "late failure", g.Samples[len(g.Samples)-1].Message)
	assert.Equal(t, maxTraceSamples+6, g.Entries)
}
