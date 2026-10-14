// Package notify turns alert-analyzer findings into notifier messages.
package notify

import (
	"fmt"
	"strings"
	"time"

	"github.com/neogan/sre-toolkit/internal/alert-analyzer/analyzer"
	"github.com/neogan/sre-toolkit/pkg/notifier"
)

const dangerRank = 3 // high and critical findings color the message red

// BuildMessage lists recommendations and flapping rules ranked at or above
// minRank (see analyzer.ParsePriority). ok is false when nothing qualifies.
// Flapping rules already named by a recommendation are not repeated.
func BuildMessage(stats analyzer.SummaryStats, recs []analyzer.Recommendation, flapping []analyzer.FlappingResult, minRank int, now time.Time) (notifier.Message, bool) {
	covered := map[string]struct{}{}
	counts := map[string]int{}
	var lines []notifier.Line
	worst := 0

	for _, r := range recs {
		rank := analyzer.PriorityRank(r.Priority)
		if rank < minRank {
			continue
		}
		if r.Target != "" {
			covered[r.Target] = struct{}{}
		}
		for _, a := range r.RelatedAlerts {
			covered[a] = struct{}{}
		}
		target := r.Target
		if target == "" {
			target = strings.Join(r.RelatedAlerts, ", ")
		}
		if target == "" {
			target = r.Category
		}
		detail := r.Summary
		if r.Action != "" {
			detail += " → " + r.Action
		}
		lines = append(lines, notifier.Line{Label: strings.ToUpper(r.Priority), Target: target, Detail: detail})
		counts[strings.ToLower(r.Priority)]++
		worst = max(worst, rank)
	}

	for _, f := range flapping {
		rank := analyzer.PriorityRank(f.Severity)
		if !f.IsFlapping || rank < minRank {
			continue
		}
		if _, dup := covered[f.AlertName]; dup {
			continue
		}
		lines = append(lines, notifier.Line{
			Label:  "FLAPPING",
			Target: f.AlertName,
			Detail: fmt.Sprintf("%.1f transitions/hour (%d transitions)", f.FlappingScore, f.TransitionCount),
		})
		counts["flapping"]++
		worst = max(worst, rank)
	}

	if len(lines) == 0 {
		return notifier.Message{}, false
	}

	breakdown := fmt.Sprintf("critical %d, high %d, medium %d, low %d, flapping %d",
		counts["critical"], counts["high"], counts["medium"], counts["low"], counts["flapping"])
	header := fmt.Sprintf(":warning: alert-analyzer: %d problematic alert rule(s) (%s)", len(lines), breakdown)
	return notifier.Message{
		Source:  "alert-analyzer",
		Time:    now.UTC(),
		Header:  header,
		Subject: fmt.Sprintf("[alert-analyzer] %d problematic alert rule(s)", len(lines)),
		Summary: fmt.Sprintf("Analyzed %d firings across %d unique alerts: %s.", stats.TotalFirings, stats.UniqueAlerts, breakdown),
		Lines:   lines,
		Danger:  worst >= dangerRank,
	}, true
}
