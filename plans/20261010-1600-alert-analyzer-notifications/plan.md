---
title: alert-analyzer Slack/email notifications + shared pkg/notifier
status: completed
priority: medium
created: 2026-10-10
---

# alert-analyzer notifications (backlog §2, L108)

Backlog: `docs/tasks/backlog.md` L108 "Slack/email notifications about problematic rules". Jira (L109) out of scope.
Prior: `plans/20261010-1200-cert-monitor-slack-email-alerts`, `plans/20261010-1400-cert-monitor-escalation-history`.

## Current state
- `internal/cert-monitor/notifier` is cert-specific: `Report{Alerts []CertAlert, Summary}` baked into Slack/Email/Webhook formatting, plus Dedup/Escalation. Transport bits are generic: `postJSON` (`http.go`), SMTP dial/send + `EmailConfig` + TLS modes (`email.go`), Slack payload (`slack.go`), `Multi` fan-out.
- alert-analyzer `analyze` (`cmd/alert-analyzer/main.go:52`, `runAnalyze` L232) → `performAnalysis` → `analysisResult{stats, topAlerts, flapping, recommendations,...}` → `reportAnalysis` to stdout. `monitor` loops every `--interval` (default 1m) w/ Prometheus metrics.
- "Problematic rules" = `analyzer.Recommendation` (priority critical/high/medium/low; category tuning/stability/dedup/review/dead_rule) + `FlappingResult.IsFlapping`.

## Design (KISS/YAGNI)
- Do NOT generalize cert `Report`. New `pkg/notifier` holds neutral transport: `Message{Source, Title, Summary string, Severity, Lines []Line, Escalated bool}`, `Sender` iface (`Send(ctx, Message)`), `Multi`, Slack/Email/Webhook senders, `postJSON`, `EmailConfig`. 
- cert-monitor `notifier` keeps `Report`/Dedup/Escalation/`BuildReport`; its Slack/Email/Webhook `Notify` become thin adapters: Report → Message → pkg sender. Public cert API + all existing tests unchanged (green gate).
- alert-analyzer: `internal/alert-analyzer/notify` builds `Message` from `[]Recommendation` (+ flapping) filtered by `--notify-on` min priority. One-shot on `analyze` only. No dedup/escalation. `monitor` excluded (1m loop = spam) — see questions.
- Flags: `--slack-webhook` (env `SLACK_WEBHOOK_URL`), `--slack-channel`, `--smtp-host/-port/-user/-from/-to/-tls` (pw env `SMTP_PASSWORD`), `--webhook`, `--notify-on` (low|medium|high|critical, default high). Same names as cert-monitor for consistency; validated before analysis (fail fast).
- Send failure: log error, exit code unaffected (match cert-monitor `dispatch`).

## Phases
| # | Phase | Status |
|---|-------|--------|
| 1 | [Extract transport to pkg/notifier](phase-01-extract-pkg-notifier.md) | done |
| 2 | [alert-analyzer notify wiring](phase-02-alert-analyzer-wiring.md) | done |
| 3 | [Docs, README roadmap, backlog, verify](phase-03-docs-backlog.md) | done |

Order strictly sequential (2 imports pkg from 1; 3 needs final flags).

## Unresolved questions
1. Also notify from `monitor`? (rec: no; if yes later, needs Dedup → port from cert-monitor)
2. Default `--notify-on`: high (rec) vs medium?
3. Include flapping rules not covered by a recommendation as extra lines? (rec: yes, as "flapping" lines, severity from `FlappingResult.Severity`)
4. Keep `Escalator` stub in cert-monitor only (rec: yes)?
5. Webhook sink for alert-analyzer in scope? (rec: yes, ~free after extraction)
