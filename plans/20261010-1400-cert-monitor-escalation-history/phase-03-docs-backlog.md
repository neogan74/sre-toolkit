# Phase 03 - Docs, backlog cleanup, verify

## Context links
- Parent: [plan.md](plan.md) | Depends: phases 01, 02
- Files: `docs/tasks/backlog.md` (§5, ~L236-262), `README.md`, `docs/` (no cert-monitor tutorial yet)

## Overview
- Date: 2026-10-10 | Priority: low | Impl: done | Review: pending

## Key Insights
- §5 headers `Scanning`/`Alerting`/`Reporting` unticked though all children `[x]`; escalation + renewal history are the only open ones; after phases 1-2 whole tool §5 complete → also header status line.

## Requirements
- Backlog: tick Escalation + Renewal history with file refs (`notifier/escalate.go`, `history/`); tick 3 group headers; mark §5 title ✅ COMPLETE.
- README cert-monitor section: escalation + history flag examples.
- Short `docs/cert-monitor-alerting.md` (or tutorial): dedup vs escalation semantics, env secrets, history file format, limitations (in-memory episodes, no file lock).

## Related code files
- modify: `docs/tasks/backlog.md`, `README.md`; create: `docs/cert-monitor-alerting.md`

## Implementation Steps
1. Edit backlog per above (verify each child really `[x]` first).
2. README + doc page; examples must match final flag names from phases 1-2.
3. Final verify: `make lint test build` (or `go build ./... && go test ./... -race`), run `cert-monitor --help`, `watch --help`, `history --help` and compare to docs.

## Todo list
- [x] backlog ticks
- [x] README
- [x] doc page
- [x] full verify

## Success Criteria
- Docs flags == `--help` output; full test suite + lint green; backlog §5 has no `[ ]`.

## Risk Assessment
- Doc drift if flag names change late → write docs last (this phase).

## Security Considerations
- Docs must show secrets via env, not literal values.

## Next steps
Implement via `/cook` or `/code` on this plan; then consider backlog §B (shared `pkg/notify` + alert-analyzer notifications).
