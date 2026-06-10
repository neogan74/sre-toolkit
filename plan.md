# SRE Toolkit — Master Plan

> **Last reviewed:** 2026-06-10. This plan was rewritten to reflect the *actual* state of the codebase. The original 26-week build-out is complete in skeleton form: all 7 tools exist, build, and have tests. The current mission is **hardening to a coherent v1.0**, not greenfield construction.

## Vision
A comprehensive set of tools for SRE specialists that solves real production problems and demonstrates full-stack engineering depth — not just YAML, but infrastructure, automation, observability, and clean Go.

## Where the project actually is

All seven tools are scaffolded, compile, and ship CLI commands. Maturity varies:

| Tool | Maturity | Summary |
|------|----------|---------|
| k8s-doctor | **Beta** | healthcheck + diagnostics + audit; JSON/Table/YAML/HTML output; strong tests |
| alert-analyzer | **Beta** | frequency/flapping/correlation analysis, recommendations, Grafana dashboard, ~82% coverage |
| config-linter | **Beta** | k8s + Terraform + Docker + Helm linting; JSON/Table output |
| chaos-load | **Beta** | HTTP load + in-traffic chaos + k8s pod-kill/node-drain/network-partition |
| cert-monitor | **Alpha** | expiry scan + k8s secrets + Prometheus metrics + webhook; thin tests, no revocation |
| log-parser | **Alpha** | tail/query/grep, format parsing, error detection; thin tests, no exporters |
| db-toolkit | **Alpha** | health + backup commands; thin tests, limited performance analysis |

Shared foundation is solid: cobra/viper CLI, zerolog logging, Prometheus metrics framework, an OpenTelemetry tracing package, golangci-lint clean, GitHub Actions CI, goreleaser cross-compilation, branch protection.

The gap between here and v1.0 is **consistency and depth**, not new tools: uneven test coverage, observability not wired uniformly, several half-finished features, and missing per-tool docs.

---

## Strategy: harden, don't expand

The decision (2026-06-10) is to **freeze new-tool development** and bring the existing seven to a uniform v1.0 bar before starting cost-optimizer, slo-gen, or any proposed tool. Rationale:

- A coherent, polished 7-tool suite is a far stronger portfolio signal than 11 half-built tools.
- The Alpha tools (cert-monitor, log-parser, db-toolkit) drag down the suite's perceived quality; closing their gaps is the highest-leverage work.
- Observability and test parity are cross-cutting wins that lift every tool at once.

### The v1.0 bar (applies to every tool)
1. Core feature set complete (per-tool acceptance criteria in `docs/tasks/backlog.md`).
2. Test coverage ≥ 70% (≥ 80% for Beta-grade tools).
3. Prometheus metrics + structured logging wired consistently.
4. A tutorial in `docs/` and godoc on public APIs.
5. Clean Trivy/gosec scan; no panics on malformed input.

---

## Phased roadmap to v1.0

Phases are ordered by leverage, not by tool. Each is a shippable increment.

### Phase A — Test & docs parity *(close the quality gap)*
Bring the Alpha tools up to a trustworthy bar and document everything.

- Raise cert-monitor, log-parser, db-toolkit coverage to ≥ 60–70%.
- Write tutorials for config-linter, cert-monitor, log-parser, db-toolkit.
- Add `CONTRIBUTING.md`, ADR template (`docs/adr/`), issue/PR templates.
- Godoc pass across exported APIs.

**Deliverable:** every tool documented and tested; no "mystery" code paths.

### Phase B — Observability parity *(cross-cutting)*
Make every tool observable the same way.

- Prometheus `--metrics` exporter mode for k8s-doctor and chaos-load (others already export).
- Wire `pkg/tracing` into command execution paths.
- Health/readiness endpoints for long-running modes (`watch`, `monitor`).

**Deliverable:** uniform metrics/traces/health story; release **v0.8.0**.

### Phase C — Feature completion for Beta tools
Finish the obvious gaps in the strongest tools.

- alert-analyzer: Slack notifications for problematic rules.
- config-linter: GitHub Actions workflow linting, custom rule engine (OPA/Rego or config-driven), SARIF output for GitHub code-scanning.
- chaos-load: real-time TUI progress + before/after comparison reports + Prometheus export.

**Deliverable:** three Beta tools reach GA quality; release **v0.9.0**.

### Phase D — Promote Alpha tools to Beta
- cert-monitor: certificate chain validation + OCSP, inventory report, Slack channel.
- log-parser: Kubernetes log source + basic anomaly detection + one exporter (Loki).
- db-toolkit: replication lag + long-running query + slow-query analyzer (PostgreSQL first).

**Deliverable:** all seven tools at Beta+; release **v0.9.5**.

### Phase E — Release engineering & v1.0
- Container images built and pushed per tool.
- SBOM generation + image signing (cosign).
- Consolidated changelog; cut **v1.0.0**.
- Distribution: Homebrew formula + Krew manifests for the kubectl-adjacent tools.

**Deliverable:** **v1.0.0** — installable, signed, documented, uniformly tested.

---

## Architecture (as built)

```
sre-toolkit/
├── cmd/                  # CLI entry points (one dir per tool)
│   ├── k8s-doctor/  alert-analyzer/  chaos-load/
│   ├── config-linter/  cert-monitor/  log-parser/  db-toolkit/
├── internal/             # tool-specific logic (healthcheck, analyzer, scanner, ...)
├── pkg/                  # shared libraries
│   ├── cli/  config/  logging/  metrics/  tracing/
│   ├── k8s/  prometheus/  alertmanager/  testing/
├── deployments/          # docker-compose dev envs, manifests
├── docs/                 # tutorials, tasks/ (backlog, phase reports), architecture.md
├── .github/              # workflows + branch-protection ruleset
├── Makefile  .golangci.yml  .goreleaser.yml  go.mod
```

### Technology stack (in use)
- **Language:** Go 1.24+ · **CLI:** cobra + viper · **Logging:** zerolog
- **Kubernetes:** client-go · **Metrics:** prometheus/client_golang · **Tracing:** OpenTelemetry (`pkg/tracing`)
- **Testing:** testing + testify, kind for integration
- **CI/CD:** GitHub Actions, golangci-lint, Trivy, Codecov, goreleaser

---

## Development principles
1. **Production-ready code** — error handling, timeouts, retries with backoff, context propagation, no panics on bad input.
2. **Observability first** — structured logs, Prometheus metrics, tracing, health endpoints everywhere.
3. **Testing** — ≥70% unit coverage, integration tests for external deps, benchmarks for hot paths.
4. **Security** — no secrets in code, secure defaults, least privilege, scanning in CI.
5. **Documentation** — godoc, per-tool tutorial, ADRs for significant decisions.
6. **Developer experience** — fast builds, clear errors, helpful `--help`, progress indicators.

---

## Out of scope until after v1.0

These are intentionally deferred. They live in `docs/tasks/backlog.md` so they aren't forgotten, but no work starts until the seven core tools ship v1.0.

- **cost-optimizer** (HIGH value) — k8s right-sizing + cloud waste detection.
- **slo-gen** — SLO target suggestion + PrometheusRule/Grafana generation + error-budget alerting.
- **incident-cli** — timeline + post-mortem + on-call integration.
- **chaos-operator** — Ansible/Operator-SDK chaos operator.
- Ecosystem: kubectl plugins, VS Code extension, Telegram bot, Grafana datasource, GitOps integration.

---

## Risks & mitigation
- **Scope creep back into new tools** → strict freeze; new-tool ideas go to the backlog's "proposed" section only.
- **Kubernetes API drift** → support last 3 minor versions; version detection; kind-based compat tests.
- **Large-cluster performance** → pagination, concurrency, benchmark gates in CI.
- **Dependency vulnerabilities** → automated scanning, regular updates, minimal dependency tree.
- **Uneven quality perception** → the v1.0 bar is applied uniformly; no tool ships GA until it clears the same checklist.

---

## Success definition

**v1.0 (this milestone)**

- All 7 tools at Beta+ quality, uniform observability, ≥70% coverage, documented, signed binaries + images.

**Post-v1.0 (3–6 months)**

- First proposed tool (cost-optimizer) underway; community adoption signals (stars, deployments); 1–2 blog posts; conference/talk submission.

---

## Next steps
1. Start **Phase A**: pick cert-monitor as the first Alpha tool to harden (smallest surface, clearest gaps).
2. Track all work against `docs/tasks/backlog.md` acceptance criteria.
3. Tag releases at each phase boundary (v0.8 → v0.9 → v0.9.5 → v1.0).
