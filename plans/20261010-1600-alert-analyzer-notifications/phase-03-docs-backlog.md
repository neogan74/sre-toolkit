# Phase 03 - Docs, README roadmap, backlog, verify

## Context links
- Parent: [plan.md](plan.md) | Depends: phases 01, 02
- Files: `docs/alert-analyzer-tutorial.md`, `docs/cert-monitor-alerting.md`, `docs/tasks/backlog.md` (L108), `README.md` (L375-392), `docs/architecture.md`

## Overview
- Date: 2026-10-10 | Priority: low | Impl: done | Review: pending

## Key Insights
- README L377 links `docs/backlog.md` (nonexistent; real: `docs/tasks/backlog.md`).
- README roadmap "Current Phase: Foundation ✅" stale; k8s-doctor, cert-monitor, alert-analyzer, chaos-load, config-linter all shipped past it.
- cert-monitor alerting doc exists → link, don't duplicate shared SMTP/Slack semantics.

## Requirements
- Tutorial: new "Notifications" section (flags, env secrets, `--notify-on`, example Slack/email, sample message, limitations: one-shot, no dedup, `monitor` not covered).
- README: alert-analyzer example with notify flags; roadmap refreshed (verify each tool's real status first from backlog/code, don't guess); fix link.
- Backlog: tick L108 with refs (`pkg/notifier`, `cmd/alert-analyzer/notify.go`); Jira L109 stays open; update "Integrations (Slack…)" Phase 4 line / Common Components shared-lib note if applicable.
- `docs/architecture.md`: mention `pkg/notifier` if it lists pkg modules.

## Related code files
- modify: docs above; create: none

## Implementation Steps
1. Verify final flag names via `alert-analyzer analyze --help`.
2. Edit tutorial, README, backlog, architecture.
3. `grep -rn "docs/backlog.md"` for other stale links.
4. Final verify: `go build ./... && go vet ./... && go test ./... -race`, lint; compare `--help` to docs.

## Todo list
- [ ] tutorial section
- [ ] README example + roadmap + link fix
- [ ] backlog ticks
- [ ] architecture note
- [ ] full verify

## Success Criteria
- Docs flags == `--help` output; no dead links; backlog L108 `[x]`; CI-equivalent commands green.

## Risk Assessment
- Roadmap claims drift from reality → derive from backlog statuses only.

## Security Considerations
- Docs show env-var secrets, never inline passwords in examples beyond placeholders.

## Next steps
Open PR (`/git:pr`); consider follow-ups: `monitor` notifications w/ dedup, Jira (L109), shared dedup in pkg.
