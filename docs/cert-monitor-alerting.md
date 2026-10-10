# cert-monitor: alerting, escalation, history

## Flow (`watch`)
```
scan -> BuildReport(--min-severity) -+-> Escalation (--escalate-after) -> escalation sinks
                                     +-> Dedup (--renotify-interval)  -> webhook/Slack/email
```
Escalation sees the full report, so dedup never hides it.

## Dedup vs escalation
| | Dedup | Escalation |
|---|---|---|
| Purpose | stop repeat spam | unresolved too long → bring in more people |
| Fires | new / status change / every `--renotify-interval` | once per episode after `--escalate-after` |
| Targets | main sinks | `--escalate-slack-webhook`, `--escalate-smtp-to` |
| Episode ends | host stops alerting | host stops alerting |

Escalated Slack/email messages are marked `ESCALATED` and show `unresolved for <duration>`.
`--escalate-after` needs at least one escalation sink; email escalation reuses `--smtp-host/-from/-user/-tls`.
A failed escalation send is retried on the next scan.

Limits: state is in-memory (restart resets timers); a host flapping OK↔alerting resets its episode;
ERROR results count as alerting.

## Secrets
Pass via environment, never literals: `SLACK_WEBHOOK_URL`, `ESCALATE_SLACK_WEBHOOK_URL`, `SMTP_PASSWORD`.

```bash
export SLACK_WEBHOOK_URL=... ESCALATE_SLACK_WEBHOOK_URL=... SMTP_PASSWORD=...
cert-monitor watch --interval 1h --escalate-after 24h \
  --smtp-host smtp.example.com --smtp-from certs@example.com --smtp-to team@example.com \
  --escalate-smtp-to oncall@example.com --history-file /var/lib/cert-monitor/history.json \
  example.com api.example.com
```

## Renewal history
`--history-file` (or `$CERT_MONITOR_HISTORY`) on `scan`/`k8s`/`watch` stores, per target
(`host:port`, or `namespace/secret` for k8s), the last good certificate and up to 50 events:
`first_seen`, `renewed` (new fingerprint with later expiry), `changed` (new fingerprint, expiry not later).
Failed scans are ignored. File is JSON, mode 0600, written atomically.

`cert-monitor history --history-file F [--events] [--host H] [--type T] [--since 720h] [-o json]`

Limits: no file locking (two processes on one file → last writer wins); endpoints behind load balancers
serving different certs per backend will show `changed` events.
