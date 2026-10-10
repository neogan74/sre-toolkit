# Phase 02 - Renewal history

## Context links
- Parent: [plan.md](plan.md) | Depends: none (sequence after 01 for flag files)
- Code: `internal/cert-monitor/scanner/scanner.go` (CertInfo), `inventory/`, `reporter/reporter.go`, `cmd/cert-monitor/main.go`

## Overview
- Date: 2026-10-10 | Priority: medium | Impl: done | Review: pending
- Persist last-seen cert per target + renewal events; `history` command to report.

## Key Insights
- `Serial`+`Fingerprint`+`NotAfter` already scanned; recording is a pure post-scan step.
- Data tiny (targets × ≤50 events) → one JSON file; atomic rename avoids corruption on kill.

## Requirements
- `--history-file <path>` on `scan`, `k8s`, `watch` (opt-in; empty = disabled). After each scan, `Record(results)`.
- Event types: `first_seen`, `renewed` (fingerprint changed and NotAfter later), `changed` (fingerprint changed, NotAfter not later e.g. rollback/reissue-shorter). Skip results with `Error != ""` / empty fingerprint (don't clobber last good).
- Event fields: time, target, type, old/new serial, old/new NotAfter, issuer.
- Cap 50 events/target (drop oldest).
- `cert-monitor history --history-file F [--host H] [--since 720h] [--type renewed] [-o table|json]` — lists events newest first; also summary per target: current NotAfter, last renewed, renewal count.
- Missing file = empty history, not error. Corrupt file → clear error, never overwritten silently (write to `.bak`? rec: return error, leave file).
- File perms 0600, dir 0700.

## Architecture
`internal/cert-monitor/history/history.go`:
```go
type Store struct{ path string; mu sync.Mutex; data File }
type File struct{ Targets map[string]*Target `json:"targets"`}
type Target struct{ Serial, Fingerprint, Issuer string; NotAfter, FirstSeen, LastSeen time.Time; Events []Event }
type Event struct{ Time time.Time; Type string; OldSerial,NewSerial string; OldNotAfter,NewNotAfter time.Time }
func Open(path) (*Store, error); func (s *Store) Record(results []*scanner.CertInfo, now time.Time) ([]Event, error)
func (s *Store) Events(f Filter) []Event; func (s *Store) Summary() []TargetSummary
```
`Record` returns new events → `watch`/`scan` log "renewal detected" at info. (Optional later: notify on renewal — NOT in scope.)
Reporter: table + json render in `history` subcommand (small, local to `cmd` or `reporter`).

## Related code files
- create: `history/history.go`, `history/history_test.go`, `cmd/cert-monitor/history.go` (+ test)
- modify: `cmd/cert-monitor/main.go` (register cmd, flag + Record call in 3 commands), maybe `reporter/reporter.go`

## Implementation Steps
1. Store: Open/Record/atomic save + tests (first seen, renewed, same cert no event, errored result skipped, cap, corrupt file, missing file, concurrent Record under -race).
2. Events/Summary filters + tests.
3. `history` cobra cmd: flags, table (tabwriter) + json; tests with temp file.
4. Shared helper `recordHistory(logger, path, results)` called from scan/k8s/watch; failure logs warn, never fails scan.
5. Lint + tests.

## Todo list
- [x] history pkg + tests
- [x] history command
- [x] wire into scan/k8s/watch
- [x] lint/test green

## Success Criteria
- Two scans against local test server whose cert is swapped between runs yield `first_seen` then `renewed`; `history` shows both; unchanged rescan adds none; `-race` clean.

## Risk Assessment
- Multi-process concurrent writers (two watchers same file) → last-writer-wins; document, no file locking (YAGNI).
- Serial reuse / load-balanced endpoints returning different certs per backend → false `changed` flapping; note in docs, per-host-port key only.

## Security Considerations
- File holds only public cert metadata (no keys); still 0600. Path from flag only; no shell/template use.

## Next steps
Phase 03.
