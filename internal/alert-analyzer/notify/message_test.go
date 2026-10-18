package notify

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/neogan/sre-toolkit/internal/alert-analyzer/analyzer"
)

var now = time.Date(2026, 10, 10, 12, 0, 0, 0, time.UTC)

func rank(t *testing.T, s string) int {
	t.Helper()
	r, err := analyzer.ParsePriority(s)
	require.NoError(t, err)
	return r
}

func TestBuildMessage_FiltersByPriority(t *testing.T) {
	recs := []analyzer.Recommendation{
		{Category: "tuning", Priority: "critical", Target: "A", Summary: "sa", Action: "fix a"},
		{Category: "review", Priority: "high", Target: "B", Summary: "sb"},
		{Category: "tuning", Priority: "medium", Target: "C", Summary: "sc"},
		{Category: "tuning", Priority: "low", Target: "D", Summary: "sd"},
	}
	stats := analyzer.SummaryStats{TotalFirings: 50, UniqueAlerts: 4}

	msg, ok := BuildMessage(stats, recs, nil, rank(t, "high"), now)
	require.True(t, ok)
	require.Len(t, msg.Lines, 2)
	assert.Equal(t, "CRITICAL", msg.Lines[0].Label)
	assert.Equal(t, "A", msg.Lines[0].Target)
	assert.Equal(t, "sa → fix a", msg.Lines[0].Detail)
	assert.Equal(t, "sb", msg.Lines[1].Detail)
	assert.True(t, msg.Danger)
	assert.Equal(t, "alert-analyzer", msg.Source)
	assert.Contains(t, msg.Header, "2 problematic alert rule(s)")
	assert.Contains(t, msg.Subject, "[alert-analyzer] 2")
	assert.Contains(t, msg.Summary, "50 firings across 4 unique alerts")

	msg, ok = BuildMessage(stats, recs, nil, rank(t, "low"), now)
	require.True(t, ok)
	assert.Len(t, msg.Lines, 4)
}

func TestBuildMessage_NothingQualifies(t *testing.T) {
	recs := []analyzer.Recommendation{{Priority: "low", Target: "A"}}
	_, ok := BuildMessage(analyzer.SummaryStats{}, recs, nil, rank(t, "high"), now)
	assert.False(t, ok)
	_, ok = BuildMessage(analyzer.SummaryStats{}, nil, nil, rank(t, "low"), now)
	assert.False(t, ok)
}

func TestBuildMessage_FlappingDedupAndThreshold(t *testing.T) {
	recs := []analyzer.Recommendation{
		{Priority: "high", Target: "covered", Summary: "s"},
		{Priority: "high", RelatedAlerts: []string{"related"}, Summary: "s"},
	}
	flap := []analyzer.FlappingResult{
		{AlertName: "covered", IsFlapping: true, Severity: "critical"},
		{AlertName: "related", IsFlapping: true, Severity: "critical"},
		{AlertName: "extra", IsFlapping: true, Severity: "critical", FlappingScore: 6.5, TransitionCount: 13},
		{AlertName: "calm", IsFlapping: false, Severity: "critical"},
		{AlertName: "weak", IsFlapping: true, Severity: "warning"},
	}
	msg, ok := BuildMessage(analyzer.SummaryStats{}, recs, flap, rank(t, "high"), now)
	require.True(t, ok)
	require.Len(t, msg.Lines, 3)
	last := msg.Lines[2]
	assert.Equal(t, "FLAPPING", last.Label)
	assert.Equal(t, "extra", last.Target)
	assert.Equal(t, "6.5 transitions/hour (13 transitions)", last.Detail)
	assert.Equal(t, "related", msg.Lines[1].Target, "related alerts used as target when Target empty")
}

func TestBuildMessage_DangerOnlyForHighAndAbove(t *testing.T) {
	recs := []analyzer.Recommendation{{Priority: "medium", Target: "A"}}
	msg, ok := BuildMessage(analyzer.SummaryStats{}, recs, nil, rank(t, "medium"), now)
	require.True(t, ok)
	assert.False(t, msg.Danger)
}

func TestBuildMessage_TargetFallsBackToCategory(t *testing.T) {
	recs := []analyzer.Recommendation{{Priority: "high", Category: "review", Summary: "s"}}
	msg, ok := BuildMessage(analyzer.SummaryStats{}, recs, nil, 1, now)
	require.True(t, ok)
	assert.Equal(t, "review", msg.Lines[0].Target)
}
