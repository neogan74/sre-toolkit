# SRE Toolkit — Product Backlog

> **Last reviewed:** 2026-06-10 — verified against source in `cmd/` and `internal/`.
> **Current focus:** Harden the 7 existing tools to a coherent **v1.0** release.

## Project Goal
A set of practical, production-grade tools for SRE specialists, demonstrating deep understanding of infrastructure, programming, and operational work.

## How to read this file
- `[x]` = implemented and present in the codebase.
- `[ ]` = not yet implemented.
- **Status** lines reflect verified code state, not aspiration.
- Each tool ends with **v1.0 acceptance criteria** — the bar for calling it "done".

---

## Status at a glance

| Tool | Commands | Tests | Maturity | Biggest v1.0 gap |
|------|----------|-------|----------|------------------|
| k8s-doctor | healthcheck, diagnostics, audit, version | 12 files | **Beta** | Prometheus metrics export, benchmarks |
| alert-analyzer | analyze, version | 12 files | **Beta** | Notifications (Slack/email), Jira |
| config-linter | lint (k8s/tf/docker/helm), version | 4 files | **Beta** | CI/CD config linting, custom rules engine |
| chaos-load | http, k8s (pod-kill/node-drain/network-partition), mock | 8 files | **Beta** | Prometheus export, real-time TUI, comparison reports |
| cert-monitor | scan, watch, monitor, version | 3 files | **Alpha** | OCSP/CRL revocation, chain validation, more tests |
| log-parser | tail, query, grep | 2 files | **Alpha** | Anomaly detection, exporters, more tests |
| db-toolkit | health, backup | 4 files | **Alpha** | Replication lag, slow-query analyzer, more tests |

Maturity legend: **Alpha** = works, thin tests / missing core features; **Beta** = feature-complete core + good tests; **GA/v1.0** = acceptance criteria below all met.

---

## Tools

### 1. k8s-doctor — Kubernetes health checker ✅ **Beta**
**Priority:** HIGH · **Complexity:** Medium
**Commands:** `healthcheck`, `diagnostics`, `audit`, `version`

#### Cluster health check ✅
- [x] API server availability check (ping with timeout)
- [x] Node status (Ready, NotReady, MemoryPressure, DiskPressure)
- [x] Critical components (etcd, controller-manager, scheduler, coredns, kube-proxy)
- [x] Node role identification (control-plane, worker)
- [x] Version tracking per node + component version compatibility validation

#### Problem diagnostics ✅
- [x] Pods in CrashLoopBackOff / ImagePullBackOff / Pending
- [x] High restart-count detection (>5 warning, >10 critical)
- [x] Container error detection (CreateContainerError, RunContainerError)
- [x] Severity classification (Critical / Warning / Info)
- [x] Resource-pressure warnings (Memory / Disk / PID / Network)
- [x] Event analysis with Warning/Error filtering
- [x] Resource limits check (CPU/Memory requests/limits)
- [x] High-load node identification (CPU/memory usage metrics)

#### Best-practices audit ✅ (`audit` command)
- [x] Liveness/readiness probe presence
- [x] Security context validation (runAsNonRoot, readOnlyRootFilesystem)
- [x] NetworkPolicies check
- [x] RBAC permissions audit (excessive permissions)
- [x] Resource quotas check per namespace

#### Reports and export
- [x] JSON / Table / YAML output (tabwriter with alignment)
- [x] HTML report output (`internal/k8s-doctor/reporter/html.go`)
- [x] CI/CD integration (exit codes on critical issues)
- [x] Emoji severity indicators + summary statistics
- [ ] Prometheus metrics export (push/exporter mode)

**v1.0 acceptance criteria**

- [ ] Prometheus exporter mode (`--metrics` flag serving `/metrics`)
- [ ] Benchmark tests for large-cluster scans (target <30s @ 100 nodes)
- [ ] Integration tests stay green in CI against kind
- [ ] Godoc on all exported types in `internal/k8s-doctor/...`

---

### 2. alert-analyzer — Prometheus/Alertmanager alert analyzer ✅ **Beta**
**Priority:** HIGH · **Complexity:** Medium-High
**Commands:** `analyze`, `version`

#### Collection and aggregation ✅
- [x] Prometheus API connection (timeout/TLS)
- [x] Alert history via `ALERTS{}` query, configurable lookback
- [x] Grouping by alert name; label extraction (severity, namespace, service)
- [x] Alertmanager API connection
- [x] Multi-source support (multi-cluster)
- [x] Victoria Metrics compatibility

#### Pattern analysis ✅
- [x] Top "noisy" alerts (highest firing count) + firing frequency
- [x] Total/unique counting; fired/resolved tracking
- [x] Flapping detection
- [x] Alert correlation (which alerts fire together)
- [x] Temporal patterns (day of week, time of day)

#### Recommendations ✅
- [x] for/threshold tuning suggestions
- [x] "Dead" rule identification (never firing)
- [x] Signal-to-noise ratio assessment
- [x] Rule prioritization for review

#### Dashboard and reports
- [x] Table output with top-N alerts
- [x] JSON export; summary statistics
- [x] Markdown report generation
- [x] Grafana dashboard with analysis metrics
- [x] Prometheus metrics framework
- [ ] Slack / email notifications about problematic rules
- [ ] Jira integration for task creation

**Coverage:** ~82% on `alert-analyzer` packages (per last run).

**v1.0 acceptance criteria**

- [ ] At least one notification channel shipped (Slack webhook preferred)
- [ ] End-to-end test against a seeded Prometheus in docker-compose
- [ ] Documented recommendation thresholds (tunable via config)

---

### 3. config-linter — Configuration linter ✅ **Beta**
**Priority:** MEDIUM · **Complexity:** Medium
**Commands:** `lint`, `version`

#### Kubernetes YAML ✅
- [x] Schema validation (client-go decoder)
- [x] Anti-patterns (latest tag, missing probes)
- [x] Security checks (privileged, hostNetwork/PID/IPC, runAsNonRoot, readOnlyRootFilesystem, allowPrivilegeEscalation, dangerous capabilities)
- [x] Resource limits requirements (CPU/Memory)
- [x] JSON / Table output

#### Terraform ✅
- [x] Provider version constraints
- [x] Open security groups (SSH/RDP/MySQL/Redis/all-traffic from 0.0.0.0/0)
- [x] Hardcoded credentials (AWS keys, secrets, tokens)
- [x] Unencrypted storage (EBS, RDS)
- [x] Public S3/GCS ACLs; local backend warning; GCP open firewall ranges

#### Docker / Containerfiles ✅
- [x] Latest tag / unpinned base image; MAINTAINER deprecated
- [x] sudo in RUN; apt-get upgrade (non-deterministic)
- [x] ADD vs COPY; non-root USER (last stage); absolute WORKDIR

#### Helm charts ✅ (`internal/config-linter/linter/helm.go`)
- [x] Template / chart linting (basic)
- [ ] Values.yaml schema check
- [ ] Dependency analysis + version compatibility

#### CI/CD configs ⏳ **NOT STARTED**
- [ ] GitHub Actions workflow validation
- [ ] GitLab CI syntax check
- [ ] Jenkins pipeline lint

**v1.0 acceptance criteria**

- [ ] Helm: values schema + dependency checks complete
- [ ] At least GitHub Actions workflow linting shipped
- [ ] Custom rule engine (OPA/Rego or config-driven) with docs
- [ ] SARIF output for GitHub code-scanning integration

---

### 4. chaos-load — Load generator and chaos testing ✅ **Beta**
**Priority:** MEDIUM · **Complexity:** High
**Commands:** `http`, `k8s` (`pod-kill`, `node-drain`, `network-partition`), `mock`

#### HTTP load generation ✅
- [x] Configurable concurrency (workers); target URL; duration-based runs
- [x] Request-limit support; HTTP methods and payloads
- [x] Authentication (Bearer, Basic)

#### Chaos scenarios (in-traffic) ✅ partial
- [x] Random HTTP 5xx errors
- [x] Network latency injection
- [x] Connection failures; timeout simulation
- [ ] Resource exhaustion (memory/CPU)

#### Kubernetes integration ✅ partial
- [x] Pod killing (graceful/force)
- [x] Node draining
- [x] Network partition between services (`network-partition` command)
- [ ] Storage issues (disk full)

#### Reporting
- [x] Summary report in terminal (latency / status codes)
- [ ] Real-time dashboard (terminal UI)
- [ ] Metrics export to Prometheus
- [ ] JMeter/Locust-compatible reports
- [ ] Comparison reports (before/after)

**v1.0 acceptance criteria**

- [ ] Prometheus metrics export during runs
- [ ] Real-time progress indicator (live RPS/latency)
- [ ] Comparison report (baseline vs run) in JSON + terminal
- [ ] Documented safety guardrails for k8s chaos (dry-run, label selectors)

---

### 5. cert-monitor — SSL/TLS certificate monitoring ⏳ **Alpha**
**Priority:** MEDIUM · **Complexity:** Low-Medium
**Commands:** `scan`, `watch`, `monitor`, `version`

#### Scanning
- [x] Expiration-date check (`internal/cert-monitor/scanner`)
- [x] Kubernetes secrets monitoring (`internal/cert-monitor/k8ssecrets`)
- [ ] Certificate chain validation
- [ ] Revocation status (OCSP / CRL)

#### Alerting
- [x] Prometheus metrics export (`cert_monitor_cert_days_left`, `cert_monitor_cert_status`, `cert_monitor_certs_total`, `cert_monitor_last_scan_timestamp_seconds`, `cert_monitor_scan_duration_seconds`)
- [x] Webhook integration (`internal/cert-monitor/notifier`)
- [ ] Email / Slack notifications
- [ ] Escalation policy

#### Reporting
- [ ] Certificate inventory
- [ ] Grouping by domain / issuer
- [ ] Renewal history tracking

**v1.0 acceptance criteria**

- [ ] Chain validation + OCSP check
- [ ] Slack notification channel
- [ ] Certificate inventory report (table + JSON)
- [ ] Test coverage ≥ 70% on scanner + notifier

---

### 6. log-parser — Smart log parser ⏳ **Alpha**
**Priority:** LOW-MEDIUM · **Complexity:** Medium-High
**Commands:** `tail`, `query`, `grep`

#### Parsing ✅ partial
- [x] Format support (`internal/log-parser/formats` — JSON/logfmt/regex)
- [x] Query + grep over parsed streams
- [ ] Kubernetes pod/container log source
- [ ] Systemd journal source

#### Analysis
- [x] Error pattern detection (`internal/log-parser/analyzer`)
- [ ] Anomaly detection (statistical/ML)
- [ ] Log correlation (trace ID)
- [ ] Performance-metric extraction

#### Visualization
- [ ] Terminal UI for live tail
- [ ] Export to Loki / Elasticsearch
- [ ] Histogram / timeline view

**v1.0 acceptance criteria**

- [ ] Kubernetes log source supported
- [ ] Basic anomaly detection (rate spikes / new error signatures)
- [ ] One export target (Loki preferred)
- [ ] Test coverage ≥ 60%

---

### 7. db-toolkit — Database operations helper ⏳ **Alpha**
**Priority:** LOW-MEDIUM · **Complexity:** Medium
**Commands:** `health`, `backup`

#### Health checks ✅ partial
- [x] Connection / health checks (`internal/db-toolkit/health`)
- [ ] Replication lag check
- [ ] Long-running query detection
- [ ] Table / index bloat analysis

#### Backup / restore ✅ partial
- [x] Backup command (`internal/db-toolkit/backup`)
- [ ] Point-in-time recovery
- [ ] Backup validation
- [ ] Cross-region replication status

#### Performance
- [x] Analyzer scaffold (`internal/db-toolkit/analyzer`)
- [ ] Slow-query analyzer
- [ ] Index recommendations
- [ ] Query explain analyzer

**v1.0 acceptance criteria**

- [ ] PostgreSQL fully supported (health, backup, slow-query)
- [ ] Replication lag + long-running query checks
- [ ] Backup validation step
- [ ] Test coverage ≥ 60% with a containerized Postgres

---

## Shared components

### CLI framework ✅ **Complete**
- [x] Cobra command structure (`pkg/cli/root.go`)
- [x] Viper configuration (`pkg/config/config.go`)
- [x] zerolog structured logging (`pkg/logging/`)
- [x] YAML/env config support
- [ ] Shared progress bars / spinners helper (used consistently across tools)

### Observability ✅ **Partial**
- [x] Prometheus metrics framework (`pkg/metrics/`)
- [x] HTTP metrics server; custom metrics (executions, duration, processed, errors)
- [x] OpenTelemetry tracing package present (`pkg/tracing/tracing.go`)
- [ ] Tracing wired into all tool commands
- [ ] Health / readiness endpoints

### Testing ✅ **Partial**
- [x] Unit framework (testing + testify); table-driven tests
- [x] Coverage reporting (`coverage.out`)
- [ ] Consistent ≥80% coverage across all tools (cert-monitor, log-parser, db-toolkit lag)
- [ ] E2E suite
- [ ] Mock generators (mockery/gomock) standardized

### DevOps / CI-CD ✅ **Mostly complete**
- [x] GitHub Actions (`.github/workflows/ci.yml`, `release.yml`)
- [x] golangci-lint (all 151 prior issues fixed), Codecov, Trivy scanning, artifact upload
- [x] Release automation via goreleaser (7 tools × linux/darwin/windows × amd64/arm64)
- [x] Branch protection ruleset (`.github/rulesets/main-protection.json`)
- [ ] Container image builds + push (per tool)
- [ ] SBOM generation + image signing (cosign)

### Documentation ✅ **Partial**
- [x] README with badges; plan.md roadmap; this backlog
- [x] Tutorials: k8s-doctor, alert-analyzer, chaos-load
- [x] PHASE1/PHASE2 completion docs; MIT License
- [ ] Tutorials for config-linter, cert-monitor, log-parser, db-toolkit
- [ ] Godoc coverage on public APIs
- [ ] Architecture Decision Records (`docs/adr/`)
- [ ] CONTRIBUTING.md + issue/PR templates

---

## Roadmap (current focus: hardening to v1.0)

### Sprint A — Close the test & docs gaps (highest leverage)
1. [ ] Raise cert-monitor, log-parser, db-toolkit coverage to ≥60–70%
2. [ ] Add tutorials for config-linter, cert-monitor, log-parser, db-toolkit
3. [ ] Add CONTRIBUTING.md, ADR template, issue/PR templates
4. [ ] Godoc pass on all exported APIs

### Sprint B — Observability parity across tools
1. [ ] Prometheus `--metrics` exporter mode for k8s-doctor, chaos-load
2. [ ] Wire `pkg/tracing` into command execution paths
3. [ ] Health/readiness endpoints in long-running modes (watch/monitor)

### Sprint C — Feature completion for Beta tools
1. [ ] alert-analyzer: Slack notifications
2. [ ] config-linter: GitHub Actions linting + custom rule engine + SARIF output
3. [ ] chaos-load: real-time TUI + comparison reports

### Sprint D — Promote Alpha tools to Beta
1. [ ] cert-monitor: chain validation + OCSP, inventory report
2. [ ] log-parser: k8s log source + basic anomaly detection
3. [ ] db-toolkit: replication lag + slow-query analyzer (Postgres)

### Sprint E — Release engineering & v1.0
1. [ ] Container images + push for all tools
2. [ ] SBOM + cosign signing
3. [ ] Cut **v1.0.0** with consolidated changelog

---

## Backlog — proposed new tools (post-v1.0, not started)

These remain ideas for after the 7 core tools reach v1.0. Kept here so they aren't lost, but explicitly out of current scope.

- **cost-optimizer** (HIGH) — k8s right-sizing (over/under-provisioned pods), unattached volumes, idle load balancers, snapshot retention. *Go, AWS/GCP SDK, k8s client.*
- **slo-gen** (MEDIUM) — analyze Prometheus history → suggest SLO targets, generate PrometheusRules + Grafana JSON, error-budget burn-rate alerting. *Go, Prometheus API.*
- **incident-cli** (MEDIUM) — timeline construction from logs/chat, post-mortem scaffolding, on-call (PagerDuty/OpsGenie) ack/resolve. *Go, PagerDuty/Slack API.*
- **chaos-operator** (MEDIUM) — Ansible/Operator-SDK chaos operator with finalizers, granular CRD status, memory-hog/pod-killer actions. *Ansible, Operator SDK, Molecule.*

### Other ideas
- [ ] kubectl plugin versions of all tools (+ Krew packaging)
- [ ] Homebrew formula
- [ ] Telegram bot for quick checks
- [ ] VS Code extension for config-linter
- [ ] Grafana datasource plugin
- [ ] GitOps integration (ArgoCD / Flux)

---

## Success metrics

**Technical quality**

- ≥80% test coverage on all Beta+ tools
- Zero high-severity security findings (Trivy/gosec)
- Sub-second startup; <50MB binary per tool

**Adoption (post-v1.0)**

- GitHub stars, weekly active users, production deployments

**Skills demonstration**

- Go depth · Kubernetes expertise · SRE best practices · clean architecture · full SDLC
