# Phase 01 - Escalation policy

## Context links
- Parent: [plan.md](plan.md) | Depends: none
- Code: `internal/cert-monitor/notifier/{notifier,dedup,report,slack,email}.go`, `cmd/cert-monitor/{main,notify}.go`

## Overview
- Date: 2026-10-10 | Priority: medium | Impl: done | Review: pending
- Alert unresolved for >= N hours in `watch` → one escalation notification to escalation sinks.

## Key Insights
- Dedup state resets on resolution = same episode boundary needed; but Dedup holds no first-seen, and must stay independent (escalation must fire even when dedup suppresses repeats).
- `Escalator` interface already fits: `Escalate(ctx, Report, age)`; `age` = max age in report? Per-host ages differ → build sub-report per call batch, pass the oldest age; per-alert age also stored in `CertAlert.UnresolvedFor`.

## Requirements
- Flags (watch only): `--escalate-after` (duration, 0=off), `--escalate-slack-webhook` (env `ESCALATE_SLACK_WEBHOOK_URL`), `--escalate-smtp-to` (StringSlice, reuses `--smtp-*` server settings).
- Validation: `--escalate-after>0` requires >=1 escalation sink; fail fast pre-scan. Sinks never logged with URLs.
- Fires once per host per episode; episode ends when host absent from alerting set (after min-severity filter) → state pruned.
- Escalation messages visibly marked (Slack/email subject/prefix "ESCALATED"), include unresolved duration.
- A sink failure logs error, doesn't stop scan loop; retry next scan if send failed (don't mark escalated on error).

## Architecture
```
results -> BuildReport(min) -> rep ─┬─> Dedup.Filter -> Multi.Notify        (existing)
                                     └─> Escalation.Check(rep) -> due alerts -> Escalate(ctx, subReport, age)
```
`notifier/escalate.go`:
```go
type Escalation struct{ After time.Duration; Target Notifier; now func() time.Time; mu; eps map[string]episode{first time.Time; escalated bool} }
func NewEscalation(after, target) *Escalation
func (e *Escalation) Escalate(ctx, r Report, _ time.Duration) error // implements Escalator; prunes, picks due, sends once
```
`Report.Escalated bool` + `CertAlert.UnresolvedFor string` (omitempty → webhook JSON back-compat). Slack/email renderers add prefix when `Escalated`.

## Related code files
- create: `notifier/escalate.go`, `notifier/escalate_test.go`
- modify: `notifier/report.go` (fields), `notifier/slack.go`, `notifier/email.go` (render), `cmd/cert-monitor/notify.go` (flags, build escalation target), `cmd/cert-monitor/main.go` (watch: call after dispatch), their tests

## Implementation Steps
1. Add `Escalated`/`UnresolvedFor` fields; renderers + tests (golden-ish string contains).
2. `Escalation` with injectable clock; table tests: below threshold, at threshold, once-only, resolve→reset→re-escalate, send error → retried, multi-host independence.
3. Flags + env fallback + validation in `notify.go`; build escalation target (Multi of slack/email-with-escalate-recipients).
4. Wire in `watch` `runScan`: after `dispatch`, `esc.Escalate(ctx, rep, 0)` with logging. Needs the BuildReport output; refactor `dispatch` to return the Report or expose build step.
5. `go vet`, `golangci-lint`, `go test ./internal/cert-monitor/... ./cmd/cert-monitor/...`.

## Todo list
- [x] report fields + renderers
- [x] escalate.go + tests
- [x] flags/validation/env
- [x] watch wiring
- [x] lint + tests green

## Success Criteria
- Unit tests above pass; manual: `watch --interval 10s --escalate-after 30s` against expiring cert (local test server) escalates exactly once, then silent; fixing cert then breaking again re-escalates.
- Webhook payload unchanged when not escalated.

## Risk Assessment
- Clock/episode drift if scans fail (ERROR status counts as alerting → escalates; acceptable, ERROR is an alert). Host flapping between ERROR/OK resets episode → missed escalation; accept, document.
- Restart loses episode state → escalation delayed by up to N h; document.

## Security Considerations
- Escalation webhook URL / SMTP pass via env, never logged or echoed in errors (reuse existing redaction in `http.go`).

## Next steps
Phase 02.
