# SRE Toolkit - Product Backlog

## Project Goal
Create a set of practical tools for SRE specialists, demonstrating deep understanding of infrastructure, programming, and operational work.

---

## Tools for Development

### 1. k8s-doctor - CLI for Kubernetes Checks ✅ **MVP COMPLETE**
**Priority:** HIGH
**Complexity:** Medium
**Status:** Phase 2 Complete (v0.1.0 ready)

#### Features:
- [x] **Cluster Health Check** ✅ **COMPLETE**
  - [x] API server availability check (Ping with timeout)
  - [x] Node status check (Ready, NotReady, MemoryPressure, DiskPressure)
  - [x] Critical components check (etcd, controller-manager, scheduler, coredns, kube-proxy)
  - [x] Node role identification (control-plane, worker)
  - [x] Version tracking per node
  - [x] Component version compatibility validation ✅ **COMPLETE**

- [x] **Problem Diagnostics** ✅ **MOSTLY COMPLETE**
  - [x] Find pods in CrashLoopBackOff, ImagePullBackOff, Pending states
  - [x] High restart count detection (>5 = warning, >10 = critical)
  - [x] Container error detection (CreateContainerError, RunContainerError)
  - [x] Severity classification (Critical/Warning/Info)
  - [x] Resource pressure warnings (Memory/Disk/PID/Network)
  - [x] Event analysis with Warning/Error filtering ✅ **COMPLETE**
  - [x] Resource limits check (CPU/Memory requests/limits) ✅ **COMPLETE**
  - [x] High-load node identification (CPU/memory usage metrics)

- [ ] **Best Practices Audit** ⏳ **PLANNED (Phase 3)**
  - [ ] Check for liveness/readiness probes
  - [ ] Security Context validation (runAsNonRoot, readOnlyRootFilesystem)
  - [ ] NetworkPolicies check
  - [ ] RBAC permissions audit (excessive permissions)
  - [ ] Resource quotas check in namespace

- [x] **Reports and Export** ✅ **PARTIAL**
  - [x] JSON/Table output formats (tabwriter with alignment)
  - [x] CI/CD integration (exit codes on critical issues)
  - [x] Emoji severity indicators (🔴 Critical, ⚠️ Warning, ℹ️ Info)
  - [x] Summary statistics
  - [ ] YAML output format
  - [ ] HTML report with charts
  - [ ] Prometheus metrics export

**Technologies:** Go, client-go, cobra, tabwriter, zerolog

**Completed Files:**
- `pkg/k8s/client.go` - Kubernetes client wrapper
- `internal/k8s-doctor/healthcheck/{nodes,pods,components}.go`
- `internal/k8s-doctor/diagnostics/diagnostics.go`
- `internal/k8s-doctor/reporter/reporter.go`
- `cmd/k8s-doctor/main.go` - CLI with healthcheck/diagnostics commands
- `docs/k8s-doctor-tutorial.md` - 400+ line user guide
- `internal/k8s-doctor/healthcheck/{nodes,pods,components}_test.go` ✅ **NEW**
- `internal/k8s-doctor/diagnostics/diagnostics_test.go` ✅ **NEW**

**Next Steps:**
- [ ] Integration tests with kind (Phase 2.5)
- [ ] Implement audit command
- [ ] Add event analysis
- [ ] Resource limits checking
- [ ] Benchmark tests for performance

---

### 2. alert-analyzer - Prometheus/Alertmanager Alert Analyzer ⏳ **PHASE 1 COMPLETE**
**Priority:** HIGH
**Complexity:** Medium-High
**Status:** Phase 1 Complete (Frequency Analysis MVP)

#### Features:
- [x] **Collection and Aggregation** ✅ **PARTIAL**
  - [x] Prometheus API connection with timeout/TLS support
  - [x] Alert history collection via `ALERTS{}` query
  - [x] Time range queries with configurable lookback
  - [x] Grouping by alert name
  - [x] Label extraction (severity, namespace, service)
  - [ ] Alertmanager API connection
  - [ ] Multi-source support (multi-cluster)

- [x] **Pattern Analysis** ✅ **PARTIAL**
  - [x] Top "noisy" alerts (highest firing count)
  - [x] Firing frequency calculation
  - [x] Total/unique alert counting
  - [x] Alert history tracking (fired/resolved times)
  - [ ] Flapping alerts detection (constantly switching)
  - [ ] Alert correlation (which alerts fire together)
  - [ ] Temporal patterns (day of week, time of day)

- [ ] **Recommendations** ⏳ **PLANNED (Phase 2)**
  - [ ] Suggestions for for/threshold tuning
  - [ ] "Dead" rule identification (never firing)
  - [ ] Signal-to-noise ratio assessment
  - [ ] Rule prioritization for review

- [x] **Dashboard and Reports** ✅ **PARTIAL**
  - [x] Table output with top-N alerts
  - [x] JSON export for automation
  - [x] Summary statistics (total/unique alerts)
  - [x] Prometheus metrics framework
  - [ ] Markdown report generation
  - [ ] Grafana dashboard with analysis metrics
  - [ ] Slack/email notifications about problematic rules
  - [ ] Jira integration for task creation

**Technologies:** Go, prometheus/client_golang, zerolog, cobra

**Completed Files:**
- `pkg/prometheus/client.go` - Prometheus client wrapper
- `internal/alert-analyzer/collector/{types,prometheus}.go`
- `internal/alert-analyzer/analyzer/frequency.go`
- `internal/alert-analyzer/reporter/reporter.go`
- `internal/alert-analyzer/storage/memory.go`
- `cmd/alert-analyzer/main.go` - CLI with analyze command
- `deployments/docker/alert-analyzer/` - Docker Compose dev environment
- `deployments/docker/alert-analyzer/README.md` - Setup guide

**Next Steps:**
- [ ] Unit tests (80%+ coverage target) 🔴 **HIGH PRIORITY**
- [ ] Flapping detection algorithm
- [ ] Alert correlation analysis
- [ ] Recommendations engine
- [ ] Grafana dashboard
- [ ] Victoria Metrics compatibility testing

---

### 3. chaos-load - Load Generator and Chaos Testing ⏳ **PHASE 1 COMPLETE**
**Priority:** MEDIUM
**Complexity:** High
**Status:** Phase 1 Complete (HTTP Load MVP)

#### Features:
- [x] **HTTP Load Generation** ✅ **COMPLETE**
  - [x] Configurable concurrency (workers)
  - [x] Target URL support
  - [x] Duration-based runs
  - [x] Request limit support
  - [ ] Various HTTP methods and payloads (currently GET only)
  - [ ] Authentication support (Bearer, Basic)

- [ ] **Chaos Scenarios**
  - [ ] Random HTTP 5xx errors
  - [ ] Network latency injection
  - [ ] Connection failures
  - [ ] Timeout simulation
  - [ ] Resource exhaustion (memory/CPU)

- [ ] **Kubernetes Integration**
  - [ ] Pod killing (graceful/force)
  - [ ] Node draining
  - [ ] Network partition between services
  - [ ] Storage issues (disk full)

- [x] **Reporting** ✅ **PARTIAL**
  - [x] Summary report in terminal (latency/status codes)
  - [ ] Real-time dashboard (terminal UI)
  - [ ] Metrics export to Prometheus
  - [ ] JMeter/Locust-compatible reports
  - [ ] Comparison reports (before/after)

**Technologies:** Go, net/http, stats/collector, cobra

**Completed Files:**
- `internal/chaos-load/http/worker.go`
- `internal/chaos-load/stats/collector.go`
- `cmd/chaos-load/main.go`
- `docs/chaos-load-tutorial.md`

**Next Steps:**
- [ ] Unit tests for worker and collector
- [ ] Support for POST/PUT methods with payloads
- [ ] Real-time Progress indicator
- [ ] Pod killing chaos scenario

---

### 4. config-linter - Configuration Linter
**Priority:** MEDIUM
**Complexity:** Medium

#### Features:
- [ ] **Kubernetes YAML**
  - Schema validation (OpenAPI)
  - Best practices (anti-patterns)
  - Security checks (privileged, hostNetwork)
  - Resource limits requirements

- [ ] **Helm Charts**
  - Template validation
  - Values.yaml schema check
  - Dependency analysis
  - Version compatibility

- [ ] **Terraform**
  - HCL syntax check
  - State file analysis
  - Provider version constraints
  - Security rules (open security groups)

- [ ] **Docker/Containerfiles**
  - Multi-stage builds recommendations
  - Base image vulnerabilities
  - Layer optimization
  - Best practices (COPY vs ADD, etc.)

- [ ] **CI/CD Configs**
  - GitHub Actions workflow validation
  - GitLab CI syntax check
  - Jenkins pipeline lint

**Technologies:** Go, yaml/json parsers, OPA/Rego for policy

---

### 5. cert-monitor - SSL/TLS Certificate Monitoring
**Priority:** MEDIUM
**Complexity:** Low-Medium

#### Features:
- [ ] **Scanning**
  - Expiration date check
  - Certificate chain validation
  - Revocation status check (OCSP/CRL)
  - Kubernetes secrets monitoring

- [ ] **Alerting**
  - Email/Slack notifications
  - Prometheus metrics (days_until_expiry)
  - Webhook integration
  - Escalation policy

- [ ] **Reporting**
  - Certificate inventory
  - Grouping by domain/issuer
  - Renewal history tracking

**Technologies:** Go, crypto/x509, cert-manager integration

---

### 6. log-parser - Smart Log Parser
**Priority:** LOW
**Complexity:** Medium-High

#### Features:
- [ ] **Parsing**
  - Format support (JSON, logfmt, regex)
  - Kubernetes logs (pod/container)
  - Systemd journal
  - Custom formats

- [ ] **Analysis**
  - Error pattern detection
  - Anomaly detection (ML-based)
  - Log correlation (trace ID)
  - Performance metrics extraction

- [ ] **Visualization**
  - Terminal UI for live tail
  - Export to Loki/Elasticsearch
  - Histogram/timeline view

**Technologies:** Go, go-elasticsearch, promtail libraries

---

### 7. db-toolkit - Database Operations Helper
**Priority:** LOW
**Complexity:** Medium

#### Features:
- [ ] **Health Checks**
  - Connection pool monitoring
  - Replication lag check
  - Long-running queries detection
  - Table/Index bloat analysis

- [ ] **Backup/Restore**
  - Automated backup scheduling
  - Point-in-time recovery
  - Backup validation
  - Cross-region replication status

- [ ] **Performance**
  - Slow query analyzer
  - Index recommendations
  - Query explain analyzer
  - Connection pooling stats

**Technologies:** Go, database/sql, pgx (PostgreSQL), go-mysql

---

### 8. chaos-operator - Ansible Chaos Operator ⏳ **NEW**
**Priority:** MEDIUM
**Complexity:** Medium-High
**Status:** Planned/Exploratory

#### Features:
- [ ] **Tool Detection**
  - Pre-flight checks for required binaries (stress-ng, etc.)
- [ ] **Granular Status**
  - Real-time feedback in CRD status
- [ ] **Safe Cleanup**
  - Finalizers for cleaning up chaos resources
- [ ] **Chaos Actions**
  - memory-hog implementation
  - pod-killer implementation

**Technologies:** Ansible, Operator SDK, Molecule

---

### 9. slo-gen - SLO Generator & Tracker ⏳ **PROPOSED**
**Priority:** MEDIUM
**Complexity:** Medium

#### Features:
- [ ] **Data Analysis**
  - [ ] Analyze Prometheus historical metrics (success/total)
  - [ ] Suggest initial SLO targets based on specific percentile performance
- [ ] **Generation**
  - [ ] Generate Terraform/OpenTofu resources for PrometheusRules
  - [ ] Generate Grafana dashboard JSON for SLO tracking
- [ ] **Reporting**
  - [ ] Error Budget burn rate calculation
  - [ ] Alert on budget exhaustion

**Technologies:** Go, Prometheus API

---

### 10. cost-optimizer - Cloud & K8s Cost Analysis ⏳ **PROPOSED**
**Priority:** HIGH
**Complexity:** Medium-High

#### Features:
- [ ] **Kubernetes Waste**
  - [ ] Identify over-provisioned pods (Requests >> Usage)
  - [ ] Identify under-provisioned pods (Throttling/OOM risks)
  - [ ] Right-sizing recommendations
- [ ] **Cloud Resources**
  - [ ] Detect unattached volumes (EBS/PD)
  - [ ] Identify idle Load Balancers
  - [ ] Old snapshot retention analysis

**Technologies:** Go, AWS SDK, GCP SDK, k8s client

---

### 11. incident-cli - Incident Management Helper ⏳ **PROPOSED**
**Priority:** MEDIUM
**Complexity:** Low-Medium

#### Features:
- [ ] **Timeline Construction**
  - [ ] Parse logs to create automatic timeline events
  - [ ] Import discussion from chat export (Slack/JSON)
- [ ] **Post-Mortem**
  - [ ] Scaffold standard Post-Mortem Markdown
  - [ ] Auto-fill detection/resolution timestamps
- [ ] **On-Call Integration**
  - [ ] Check who is on-call (PagerDuty/OpsGenie)
  - [ ] Ack/Resolve alerts via CLI

**Technologies:** Go, PagerDuty API, Slack API

---

## Common Components

### Shared Libraries
- [x] **CLI Framework** ✅ **COMPLETE**
  - [x] Cobra-based command structure (`pkg/cli/root.go`)
  - [x] Viper for configuration (`pkg/config/config.go`)
  - [x] Logging with zerolog (`pkg/logging/`)
  - [x] Structured configuration (YAML/env support)
  - [x] Progress bars and spinners (Ready for implementation)

- [x] **Observability** ✅ **PARTIAL**
  - [x] Prometheus metrics framework (`pkg/metrics/`)
  - [x] Structured logging (zerolog)
  - [x] HTTP metrics server
  - [x] Custom metrics (command_executions, command_duration, resources_processed, errors)
  - [ ] OpenTelemetry tracing
  - [ ] Health/ready endpoints

- [x] **Testing** ✅ **PARTIAL**
  - [x] Unit test framework (testing + testify)
  - [x] Test coverage reporting (cover.out)
  - [x] Table-driven tests
  - [ ] Integration tests with real clusters
  - [ ] E2E test suite
  - [ ] Mock generators (gomock/mockery)

### DevOps
- [x] **CI/CD** ✅ **COMPLETE**
  - [x] GitHub Actions workflows (`.github/workflows/ci.yml`)
  - [x] Automated testing (lint, test, build)
  - [x] golangci-lint integration (v6)
  - [x] Codecov coverage reporting
  - [x] Trivy security scanning
  - [x] Artifact upload
  - [ ] Release automation (goreleaser)
  - [ ] Container image builds

- [x] **Documentation** ✅ **PARTIAL**
  - [x] README with examples and badges
  - [x] plan.md (26-week roadmap)
  - [x] backlog.md (feature tracking)
  - [x] k8s-doctor tutorial (400+ lines)
  - [x] alert-analyzer tutorial (600+ lines) ✨ **NEW**
  - [x] alert-analyzer README (Docker Compose setup)
  - [x] PHASE1/PHASE2 completion docs
  - [x] MIT License
  - [ ] Godoc documentation
  - [ ] Architecture Decision Records (ADR)
  - [ ] Contributing guide

---

## Priority-Based Roadmap

### Phase 1 - Foundation ✅ **COMPLETE**
1. [x] Set up Go module structure
2. [x] Create basic CLI framework (cobra, viper, zerolog)
3. [x] Implement k8s-doctor (basic health checks)
4. [x] CI/CD pipeline (GitHub Actions)
5. [x] Makefile build system
6. [x] golangci-lint configuration
7. [x] Project documentation

**Status:** COMPLETE (Dec 2024)
**Deliverables:** Working skeleton, CI pipeline, metrics framework, logging

### Phase 2 - Core Tools ⏳ **IN PROGRESS**
1. [x] k8s-doctor MVP (healthcheck, diagnostics commands)
2. [x] alert-analyzer Phase 1 (frequency analysis)
3. [x] k8s-doctor unit tests (84% coverage) ✅ **COMPLETE**
4. [x] chaos-load Phase 1 (HTTP load MVP) ✅ **COMPLETE**
5. [ ] k8s-doctor integration tests (kind)
6. [ ] alert-analyzer unit tests
7. [ ] chaos-load unit tests
8. [ ] k8s-doctor audit command

**Status:** 65% COMPLETE
**Current Focus:** Testing & Production Readiness

### Phase 3 - Advanced (Month 4-6)
1. [ ] chaos-load
2. [ ] log-parser
3. [ ] db-toolkit
4. [ ] config-linter (extension for Terraform, Dockerfile)
5. [ ] Advanced reporting (HTML, Grafana dashboards)

### Phase 4 - Polish (Month 6+)
1. [ ] Web UI for tools
2. [ ] Integrations (Slack, PagerDuty, Jira)
3. [ ] Kubernetes operator versions
4. [ ] kubectl plugins
5. [ ] Krew package manager
6. [ ] SaaS version with multi-tenancy

---

## Success Metrics

- **Technical Quality**
  - 80%+ test coverage
  - Zero high-severity security issues
  - Sub-second startup time
  - < 50MB binary size

- **Adoption**
  - GitHub stars > 100
  - Weekly active users
  - Production deployments

- **Skills Demonstration**
  - Shows Go knowledge
  - Kubernetes expertise
  - SRE best practices
  - Clean architecture

---

## Additional Ideas

- [ ] **kubectl plugin** versions of all tools
- [ ] **Telegram bot** for quick checks
- [ ] **VS Code extension** for config linting
- [ ] **Grafana datasource plugin** for integration
- [ ] **Prometheus exporter** mode
- [ ] **GitOps integration** (ArgoCD/Flux)
