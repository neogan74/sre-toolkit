# Phase 01 - Notifier core

## Context links
- Parent: [plan.md](plan.md)
- Code: `internal/cert-monitor/notifier/webhook.go`, `webhook_test.go`, `scanner/scanner.go` (Status consts)
- Docs: `docs/tasks/backlog.md` §5

## Overview
- Date: 2026-10-10 | Priority: medium | Impl: pending | Review: pending
- Extract alert building from webhook into shared code; add interface, severity filter, multi-sink.

## Key Insights
- `Notify` mixes build + send; Slack/email need same alert list + summary.
- REVOKED counted under Critical in summary today; keep.
- Severity rank: WARNING < CRITICAL=REVOKED < EXPIRED; ERROR separate (scan failure), included by default.

## Requirements
- Functional: `Notifier` iface; `BuildReport`; `--min-severity` (warning|critical|expired, default warning); chain-invalid alerts; `Multi`.
- Non-functional: webhook JSON byte-compatible; no new deps; ctx-aware.

## Architecture
```go
type Report struct { Timestamp time.Time; Source string; Alerts []CertAlert; Summary AlertSummary }
type Notifier interface { Name() string; Notify(ctx context.Context, r Report) error }
func BuildReport(results []*scanner.CertInfo, min Severity) Report // empty Alerts => caller skips
type Multi []Notifier // Notify fans out sequentially, errors.Join
```
`CertAlert` gains optional `ChainError string json:"chain_error,omitempty"` (additive).

## Related code files
- modify: `notifier/webhook.go`; `notifier/webhook_test.go`
- create: `notifier/notifier.go` (iface, Report, Severity, Multi), `notifier/report.go` (BuildReport), tests

## Implementation Steps
1. Move `CertAlert`/`AlertSummary` + build loop to `report.go`; add Severity parse/rank.
2. Add chain-invalid alert for OK-status certs with `!ChainValid && ChainError != ""` (resolve Q1 first).
3. `WebhookNotifier.Notify(ctx, Report)`; keep `NewWebhookNotifier`. Keep a thin compat helper so old call sites compile until phase 3.
4. `Multi` + `Name()` on webhook.
5. Update tests.

## Todo list
- [ ] Severity type + parse
- [ ] BuildReport + tests (each status, filter, chain-invalid, empty)
- [ ] Webhook refactor, payload golden test
- [ ] Multi test (one sink fails, others still run)

## Success Criteria
- Existing webhook tests pass unchanged in payload; `go test ./internal/cert-monitor/...` green; filter excludes below-threshold.

## Risk Assessment
- Payload drift breaks consumers -> golden JSON test.
- Chain-invalid noise for internal CAs -> only alert when chain check enabled/not `--insecure`.

## Security Considerations
- Error strings may leak internal hostnames to external sinks; document.

## Next steps
Phase 02.
