---
title: cert-monitor escalation policy + renewal history
status: completed
priority: medium
created: 2026-10-10
---

# cert-monitor: escalation + renewal history (finish backlog §5)

Backlog: `docs/tasks/backlog.md` §5 — open items: Escalation policy, Renewal history tracking; section headers (Scanning/Alerting/Reporting) still unticked though all children done.
Prior plan: `plans/20261010-1200-cert-monitor-slack-email-alerts` (notifier core, Dedup, `Escalator` hook stub).

## Current state
- `notifier.Escalator{Escalate(ctx, Report, age)}` declared (`notifier/notifier.go:19`), no impl, never called.
- `notifier.Dedup` in-memory, keyed by Host; prunes when cert stops alerting. Gives us "episode" notion but not first-seen time.
- `watch` loop in `cmd/cert-monitor/main.go` (~L304-380) calls `dispatch(ctx, logger, sinks, nopts, dedup, results)`; flags via `notify.addFlags` (`cmd/cert-monitor/notify.go`).
- `CertInfo` already has `Serial`, `Fingerprint`, `NotAfter` → enough to detect renewals. Nothing persisted.

## Design (KISS/YAGNI)
- Escalation: new `notifier.Escalation` (in-memory) tracks first-alerting time per host; once alert age >= `--escalate-after`, send ONCE per episode to escalation sinks (2nd Slack webhook / extra SMTP recipients). Report flagged `Escalated`, text says "ESCALATED, unresolved for Xh". Runs on unfiltered report (independent of dedup). Reset when host stops alerting. Implements existing `Escalator` interface. `watch` only.
- History: stdlib JSON file, atomic write (tmp+rename), opt-in `--history-file`. Per-target last-seen cert + capped event log. Renewal = serial/fingerprint changed AND NotAfter later. New `history` command reads it. No SQLite (no dep, tiny data).
- No cross-restart escalation state (in-memory), consistent with dedup decision.

## Phases
| # | Phase | Status |
|---|-------|--------|
| 1 | [Escalation policy + watch wiring](phase-01-escalation-policy.md) | done |
| 2 | [History store + recording + `history` cmd](phase-02-renewal-history.md) | done |
| 3 | [Docs, README, backlog cleanup, final verify](phase-03-docs-backlog.md) | done |

Order: 1 and 2 independent (both touch main.go/notify.go flags → do sequentially to avoid conflicts); 3 last.

## Unresolved questions
1. Escalation target: second Slack webhook + extra SMTP recipients enough, or also generic webhook / separate severity bump? (rec: Slack webhook + SMTP-to only)
2. Escalate once per episode, or repeat every N hours? (rec: once; repeat = YAGNI, renotify already covers)
3. History default: opt-in `--history-file` vs default path `~/.cert-monitor/history.json`? (rec: opt-in; `history` cmd defaults to env `CERT_MONITOR_HISTORY` if set)
4. Record history for `k8s` secrets too? Key = secret ns/name? Check what `CertInfo.Host` holds for k8s results. (rec: yes, key = Host as-is)
5. Event retention cap (rec: 50 events/target, no time-based pruning)
