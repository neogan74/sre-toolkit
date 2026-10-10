# Phase 02 - Slack + Email notifiers

## Context links
- Parent: [plan.md](plan.md) | Depends: [phase-01](phase-01-notifier-core.md)
- Code: `notifier/webhook.go` (HTTP pattern to mirror)

## Overview
- Date: 2026-10-10 | Priority: medium | Impl: pending | Review: pending
- Add `SlackNotifier` and `EmailNotifier` implementing `Notifier`.

## Key Insights
- Slack incoming webhook = JSON POST, same transport as webhook; share an http helper (DRY).
- `net/smtp` suffices; supports PLAIN auth + STARTTLS; implicit TLS (465) needs `tls.Dial` path.

## Requirements
- Slack: attachment/blocks, color by worst severity (warning=yellow, critical/revoked/expired=red), one line per alert, summary header; truncate to ~40 alerts ("+N more") for Slack limits.
- Email: multi-recipient, subject `[cert-monitor] N certificate alerts (worst: CRITICAL)`, plain-text table body (+ optional HTML skipped, YAGNI).
- Timeouts honored; ctx cancel.

## Architecture
```go
type SlackNotifier struct{ URL string; Channel, Username string; client *http.Client }
type EmailConfig struct{ Host string; Port int; Username, Password, From string; To []string; TLS string /* none|starttls|tls */ }
```
Email sender behind small `sendMail func(addr string, a smtp.Auth, from string, to []string, msg []byte) error` field for test injection.

## Related code files
- create: `notifier/slack.go`, `notifier/email.go`, `notifier/http.go` (shared post-JSON), tests (`httptest` server for Slack; in-process fake SMTP or injected sendMail for email)
- modify: `notifier/webhook.go` to use shared http helper

## Implementation Steps
1. Extract `postJSON(ctx, client, url, body)`.
2. Slack formatter + notifier; tests assert payload, color, truncation, non-2xx error.
3. Email message builder (RFC 5322 headers, CRLF, header-injection sanitize of subject/from/to); transport w/ STARTTLS default.
4. Tests: message content, auth, error paths.

## Todo list
- [ ] postJSON helper
- [ ] Slack notifier + tests
- [ ] Email notifier + tests
- [ ] Header-injection test

## Success Criteria
- Slack payload accepted by httptest; email test captures expected headers/body; coverage >= existing notifier pkg.

## Risk Assessment
- SMTP servers vary (TLS modes) -> explicit `--smtp-tls` flag, default starttls.
- Slack rate limit / size -> truncation.

## Security Considerations
- Password/webhook URL never logged (current code logs webhook URL; stop for Slack since URL is a secret).
- Reject CR/LF in addresses/subject. Refuse plain auth over non-TLS unless explicitly `none` + warn.

## Next steps
Phase 03.
