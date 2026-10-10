---
title: cert-monitor Slack + Email alerting
status: completed
priority: medium
created: 2026-10-10
---

# cert-monitor: Slack + Email notifications

Backlog: `docs/tasks/backlog.md` §5 Alerting (Email/Slack, Escalation policy).
Goal: alert on expiring / expired / revoked / error / chain-invalid certs via Slack + SMTP, reusing the webhook pattern, wired into `scan`, `k8s`, `watch`.

## Current state
- `internal/cert-monitor/notifier/webhook.go`: `WebhookNotifier.Notify(ctx, []*scanner.CertInfo)`; alert-building logic embedded in it (non-OK filter, summary counts).
- `cmd/cert-monitor/main.go`: `--webhook` duplicated in 3 commands (scan ~L142, k8s ~L269, watch ~L367), each does `NewWebhookNotifier(...).Notify`.
- Statuses: OK/WARNING/CRITICAL/EXPIRED/ERROR/REVOKED. `ChainValid/ChainError` on CertInfo but not alerted today.
- `watch` re-sends every interval => spam without dedup.

## Design (KISS/YAGNI)
- `Notifier` interface `{Name() string; Notify(ctx, Report) error}`; shared `BuildReport(results, minSeverity)` extracted from webhook.
- `Multi` fan-out: one failing sink never blocks others; errors joined.
- Webhook payload unchanged (back-compat).
- Stdlib only: Slack = incoming-webhook POST; Email = `net/smtp` (+STARTTLS). No new deps.
- Dedup in-memory, `watch` only. Escalation = hook interface only, no impl.

## Phases
| # | Phase | Status |
|---|-------|--------|
| 1 | [Notifier interface, severity filter, multi-sink](phase-01-notifier-core.md) | done |
| 2 | [Slack + Email notifiers](phase-02-slack-email.md) | done |
| 3 | [CLI wiring, dedup, escalation hook, docs/backlog](phase-03-wiring-dedup-docs.md) | done |

Order: 1 -> 2 -> 3 (2 depends on 1; 3 on both).

## Unresolved questions
1. Alert on chain-invalid when expiry is OK? (rec: yes, status `CHAIN_INVALID` in alert only, scanner status untouched)
2. Secrets: env-only (`SMTP_PASSWORD`, `SLACK_WEBHOOK_URL`) vs flags too? (rec: env fallback + flag; never log values)
3. Dedup persistence across `watch` restarts needed? (rec: no, in-memory)
4. `--config` file exists as flag but no ReadInConfig in main.go: add config-file support here or leave flags/env only? (rec: leave out)
5. Send "recovered" messages? (rec: Slack/email yes, behind `--notify-recovery`, phase 3 optional)
