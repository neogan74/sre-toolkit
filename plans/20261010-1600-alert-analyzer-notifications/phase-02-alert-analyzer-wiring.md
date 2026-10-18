# Phase 02 - alert-analyzer notification wiring

## Context links
- Parent: [plan.md](plan.md) | Depends: phase 01
- Code: `cmd/alert-analyzer/{main,analysis}.go`, `internal/alert-analyzer/analyzer/{recommendations,flapping}.go`, pattern ref `cmd/cert-monitor/notify.go`

## Overview
- Date: 2026-10-10 | Priority: medium | Impl: done | Review: pending
- After `analyze`, send problematic rules (recommendations + flapping) to Slack/email/webhook.

## Key Insights
- `analysisResult` already carries `recommendations` and `flapping`; hook goes after `reportAnalysis` in `runAnalyze` (`main.go:~262`). No analyzer changes.
- Recommendations only generated when `--show-recommendations`; notify needs them → when notify sinks configured, force recommendations (and flapping) on, or error. Rec: auto-enable w/ debug log.
- Priority weights exist (`priorityWeight`, unexported) → export or add `ParsePriority`/`Rank` in analyzer pkg for `--notify-on`.

## Requirements
- Flags on `analyze` only: `--slack-webhook` (env `SLACK_WEBHOOK_URL`), `--slack-channel`, `--webhook`, `--smtp-host|port|user|from|to|tls` (pw env `SMTP_PASSWORD`), `--notify-on` (low|medium|high|critical; default high).
- Validate pre-analysis (bad severity, smtp-host w/o from/to, bad tls) → fail fast, no network.
- Message: title "N problematic alert rule(s)", summary (alerts analyzed, lookback, recs by priority), one line per rec: `[priority] target — summary → action` (truncate: cap 40 lines Slack, "…and N more"). Severity = worst priority (critical→danger color).
- Flapping rules with `IsFlapping` and no matching rec target → extra "flapping" lines.
- Nothing >= threshold → send nothing (log info).
- Send error → log, exit 0 (analysis succeeded); no sinks → no-op.
- `monitor` unchanged.

## Architecture
```
runAnalyze → performAnalysis → reportAnalysis → notifyOpts.dispatch(ctx, logger, result)
                                                  └ notify.BuildMessage(recs, flapping, stats, min) → notifier.Multi.Send
```
`internal/alert-analyzer/notify/message.go` (pure, unit-testable); `cmd/alert-analyzer/notify.go` (flags, `build()`, `dispatch()`) mirrors cert-monitor.

## Related code files
- create: `internal/alert-analyzer/notify/{message.go,message_test.go}`, `cmd/alert-analyzer/{notify.go,notify_test.go}`
- modify: `cmd/alert-analyzer/main.go` (addFlags, runAnalyze), `internal/alert-analyzer/analyzer/recommendations.go` (export priority rank)

## Implementation Steps
1. Export priority parse/rank in analyzer pkg + test.
2. `notify.BuildMessage` + table-driven tests (threshold filter, truncation, flapping dedupe by target, empty → no message).
3. `cmd/alert-analyzer/notify.go`: opts, `addFlags`, `build()` (fail-fast), `dispatch()`.
4. Wire into `newAnalyzeCmd` + `runAnalyze`; force-enable recs/flapping when sinks set.
5. Tests: httptest Slack receives expected payload from stubbed `analysisResult`; flag validation errors; env fallbacks.
6. Manual smoke: run against test Prometheus/httptest + `--slack-webhook` to local listener.

## Todo list
- [ ] priority rank export
- [ ] BuildMessage + tests
- [ ] flags/build/dispatch
- [ ] wire analyze
- [ ] cmd tests (httptest)
- [ ] smoke run

## Success Criteria
- `alert-analyzer analyze ... --slack-webhook URL --notify-on high` posts one message listing only high+ recs.
- Invalid flags fail before any Prometheus call; default behavior (no notify flags) byte-identical output.
- `go test ./... -race` green; new code coverage >= 80%.

## Risk Assessment
- Message too long (Slack) → cap + "more" line.
- Auto-enabling recs changes stdout output → only when sinks configured; document.
- Flag name collisions in cobra (`--webhook` etc.) → none existing; check `main_test.go`.

## Security Considerations
- Secrets via env; never log URLs/passwords; alert/rule names are untrusted text → escape Slack mrkdwn specials (`<`, `>`, `&`) and strip CR/LF in email subject/header to prevent header injection.

## Next steps
Phase 03 docs.
