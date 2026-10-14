# Phase 01 - Extract transport to pkg/notifier

## Context links
- Parent: [plan.md](plan.md) | Depends: none
- Code: `internal/cert-monitor/notifier/{notifier,http,slack,email,webhook}.go` (+ tests), `cmd/cert-monitor/notify.go`

## Overview
- Date: 2026-10-10 | Priority: medium | Impl: done | Review: pending
- Move tool-agnostic delivery (Slack, SMTP, webhook, HTTP post, fan-out) to `pkg/notifier`; cert-monitor keeps domain types as adapters.

## Key Insights
- Slack/Email/Webhook formatting reads `Report.Alerts []CertAlert` → can't move as-is. Split "what to say" (Message) from "how to send" (Sender).
- Secrets: Slack URL stripped from errors by `unwrapURLError`; keep that behavior + tests.
- cert `Name()` strings ("slack", "email", "webhook") are used in Escalation `markSent` keys → keep stable.

## Requirements
- `pkg/notifier`: `Message`, `Severity` (info/warning/critical), `Sender`, `Multi`, `NewSlackSender(url, channel, username, timeout)`, `NewEmailSender(EmailConfig)`, `NewWebhookSender`, `EmailConfig.Validate`, TLS consts.
- Slack username/header prefix come from `Message.Source` (cert-monitor → "cert-monitor", analyzer → "alert-analyzer").
- cert-monitor public API (`NewSlackNotifier`, `NewEmailNotifier`, `EmailConfig`, `TLS*`, `Multi`) unchanged or aliased so `cmd/cert-monitor` + all current tests compile untouched (type aliases ok).
- Byte-for-byte same Slack payload / email body for cert alerts (assert via existing tests).

## Architecture
```
cert Report ──adapter──> pkg/notifier.Message ──> Sender(Slack|Email|Webhook)
analyzer recs ─builder─> pkg/notifier.Message ──┘
```
Webhook JSON keeps cert-specific payload shape: cert adapter marshals own body via generic `PostJSON(ctx, client, url, v any)` exported helper; analyzer uses a different `Message`-shaped body.

## Related code files
- create: `pkg/notifier/{notifier,message,http,slack,email,webhook}.go` + tests (move/port from cert-monitor tests)
- modify: `internal/cert-monitor/notifier/{slack,email,webhook,notifier}.go`, maybe `cmd/cert-monitor/notify.go` (imports only)

## Implementation Steps
1. Baseline: `go test ./internal/cert-monitor/... ./cmd/cert-monitor/... -race` green; record.
2. Create `pkg/notifier` with http.go (`PostJSON`), email transport (`EmailConfig`, dial/send, TLS) moved verbatim, generic `Message` + formatters (Slack attachment, plain-text email body).
3. Rewrite cert slack/email/webhook `Notify` to build `Message` from `Report` and delegate; keep `alertDetail` in cert pkg.
4. Port transport tests (httptest Slack, fake SMTP server, error-URL scrubbing) to `pkg/notifier`; keep cert tests asserting payload parity.
5. `go build ./... && go vet ./... && go test ./... -race`; `golangci-lint run` if configured.

## Todo list
- [ ] baseline green
- [ ] pkg/notifier skeleton + Message
- [ ] move http/email/slack/webhook transport
- [ ] cert adapters delegate
- [ ] tests ported, parity asserted
- [ ] full build/test/lint

## Success Criteria
- Zero changes needed in `cmd/cert-monitor` tests; all cert-monitor tests pass unchanged.
- `pkg/notifier` coverage >= 80%; no import of `internal/*`.
- No URL/password in any error string (tests).

## Risk Assessment
- Payload drift in cert alerts → mitigated by parity tests (golden JSON/email body).
- Over-abstracting Message → keep fields minimal; only add what both tools need.
- Import cycle: pkg must not import internal → ensured by design.

## Security Considerations
- Preserve URL scrubbing; never log webhook URL / SMTP password; keep `//nolint:gosec` rationale on user-configured URL.

## Next steps
Phase 02 consumes `pkg/notifier`.
