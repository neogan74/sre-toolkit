# Phase 03 - CLI wiring, dedup, escalation hook, docs

## Context links
- Parent: [plan.md](plan.md) | Depends: phases 01, 02
- Code: `cmd/cert-monitor/main.go` (scan/k8s/watch), `docs/tasks/backlog.md`, `README.md`

## Overview
- Date: 2026-10-10 | Priority: medium | Impl: pending | Review: pending
- One shared flag set + builder for all 3 commands; dedup for `watch`; escalation hook stub; docs.

## Key Insights
- Flags are copy-pasted per command; add `addNotifyFlags(cmd, *notifyOpts)` + `buildNotifier(opts, timeout) (notifier.Notifier, error)`.
- `watch` loops: alerting same cert each interval spams.

## Requirements
- Flags: `--slack-webhook`, `--slack-channel`, `--smtp-host`, `--smtp-port`, `--smtp-user`, `--smtp-from`, `--smtp-to` (StringSlice), `--smtp-tls`, `--min-severity`, `--renotify-interval` (watch, default 24h), `--notify-recovery` (optional).
- Env fallback: `SLACK_WEBHOOK_URL`, `SMTP_PASSWORD` (password flag-less).
- Validation: smtp-host requires from+to; fail fast before scanning.
- Dedup: key = host:port(+secret ns/name) ; send if new, status worsened/changed, or renotify elapsed; in-memory map.
- Escalation hook: `type Escalator interface{ Escalate(ctx, Report, age time.Duration) error }` declared + nil default; no impl (backlog "Escalation policy" stays open).

## Architecture
```
results -> BuildReport(min) -> [watch only] Dedup.Filter -> Multi{webhook,slack,email}.Notify
```
`notifier/dedup.go`: `Dedup{ mu; seen map[key]entry{status,lastSent}; renotify }`, `Filter(Report) Report`.

## Related code files
- modify: `cmd/cert-monitor/main.go`; `README.md`; `docs/tasks/backlog.md`
- create: `cmd/cert-monitor/notify.go`, `notifier/dedup.go`, `notifier/dedup_test.go`, `cmd/cert-monitor/notify_test.go`; optionally `docs/cert-monitor-alerting.md`

## Implementation Steps
1. notify.go: opts struct, flag registration, env fallback, validation, builder.
2. Replace 3 webhook blocks with builder + Multi (log sink names, not URLs).
3. Dedup + tests (new, repeat suppressed, status change, renotify elapsed, recovery).
4. Escalator interface placeholder + doc note.
5. Docs: usage examples, secret handling; backlog: tick Email/Slack, keep Escalation open.

## Todo list
- [ ] notify.go + validation tests
- [ ] Replace webhook blocks in scan/k8s/watch
- [ ] Dedup + tests
- [ ] Escalator stub
- [ ] README + backlog + docs
- [ ] `make lint test`

## Success Criteria
- `scan --slack-webhook X --min-severity critical` posts only >=critical; `watch` sends once then silent until change/renotify; invalid SMTP flag combo errors before scan; existing `--webhook` behavior unchanged.

## Risk Assessment
- Behavior change if chain-invalid alerts enabled by default -> consider opt-in `--alert-chain`.
- Dedup memory growth: bounded by cert count; prune entries not in latest scan.

## Security Considerations
- No secrets in logs/flags history (password env only); document file perms for k8s secret mounting.

## Next steps
Follow-up: escalation policy impl (time-based tier, e.g. Slack -> email -> PagerDuty), persisted dedup state.
