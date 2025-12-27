# SRE Toolkit - Product Backlog

## Project Goal
Create a set of practical tools for SRE specialists, demonstrating deep understanding of infrastructure, programming, and operational work.

---

## Tools for Development

### 1. k8s-doctor - CLI for Kubernetes Checks
**Priority:** HIGH
**Complexity:** Medium

#### Features:
- [ ] **Cluster Health Check**
  - API server availability check
  - Node status check (Ready, NotReady, MemoryPressure, DiskPressure)
  - Critical components check (etcd, controller-manager, scheduler)
  - Component version and compatibility validation

- [ ] **Problem Diagnostics**
  - Find pods in CrashLoopBackOff, ImagePullBackOff, Pending states
  - Event analysis with Warning/Error filtering
  - Resource limits check (CPU/Memory requests/limits)
  - High-load node identification

- [ ] **Best Practices Audit**
  - Check for liveness/readiness probes
  - Security Context validation (runAsNonRoot, readOnlyRootFilesystem)
  - NetworkPolicies check
  - RBAC permissions audit (excessive permissions)
  - Resource quotas check in namespace

- [ ] **Reports and Export**
  - JSON/YAML/Table output formats
  - HTML report with charts
  - CI/CD integration (exit codes)
  - Prometheus metrics export

**Technologies:** Go, client-go, cobra, tablewriter

---

### 2. alert-analyzer - Prometheus/Alertmanager Alert Analyzer
**Priority:** HIGH
**Complexity:** Medium-High

#### Features:
- [ ] **Collection and Aggregation**
  - Alertmanager API connection
  - Alert history collection (via Prometheus API)
  - Grouping by labels (severity, namespace, service)
  - Multi-source support (multi-cluster)

- [ ] **Pattern Analysis**
  - Top "noisy" alerts (highest firing count)
  - Flapping alerts detection (constantly switching)
  - Alert correlation (which alerts fire together)
  - Temporal patterns (day of week, time of day)

- [ ] **Recommendations**
  - Suggestions for for/threshold tuning
  - "Dead" rule identification (never firing)
  - Signal-to-noise ratio assessment
  - Rule prioritization for review

- [ ] **Dashboard and Reports**
  - Markdown report generation
  - Grafana dashboard with analysis metrics
  - Slack/email notifications about problematic rules
  - Jira integration for task creation

**Technologies:** Go, prometheus/client_golang, charts/graphs library

---

### 3. chaos-load - Load Generator and Chaos Testing
**Priority:** MEDIUM
**Complexity:** High

#### Features:
- [ ] **HTTP Load Generation**
  - Configurable RPS (requests per second)
  - Various HTTP methods and payloads
  - Authentication support (Bearer, Basic)
  - Metrics: latency (p50/p95/p99), success rate, errors

- [ ] **Chaos Scenarios**
  - Random HTTP 5xx errors
  - Network latency injection
  - Connection failures
  - Timeout simulation
  - Resource exhaustion (memory/CPU)

- [ ] **Kubernetes Integration**
  - Pod killing (graceful/force)
  - Node draining
  - Network partition between services
  - Storage issues (disk full)

- [ ] **Reporting**
  - Real-time dashboard (terminal UI)
  - Metrics export to Prometheus
  - JMeter/Locust-compatible reports
  - Comparison reports (before/after)

**Technologies:** Go, vegeta/fasthttp, chaos-mesh/litmus integration

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

## Common Components

### Shared Libraries
- [ ] **CLI Framework**
  - Cobra-based command structure
  - Viper for configuration
  - Logging (zerolog/zap)
  - Progress bars and spinners

- [ ] **Observability**
  - Prometheus metrics
  - OpenTelemetry tracing
  - Structured logging
  - Health/ready endpoints

- [ ] **Testing**
  - Unit test framework
  - Integration tests
  - E2E test suite
  - Mock generators

### DevOps
- [ ] **CI/CD**
  - GitHub Actions workflows
  - Automated testing
  - Release automation
  - Container image builds

- [ ] **Documentation**
  - README with examples
  - Godoc documentation
  - Usage tutorials
  - Architecture docs

---

## Priority-Based Roadmap

### Phase 1 - Foundation (Month 1-2)
1. Set up Go module structure
2. Create basic CLI framework
3. Implement k8s-doctor (basic health checks)
4. CI/CD pipeline

### Phase 2 - Core Tools (Month 2-4)
1. k8s-doctor (full version)
2. alert-analyzer
3. cert-monitor
4. config-linter (k8s/helm)

### Phase 3 - Advanced (Month 4-6)
1. chaos-load
2. log-parser
3. db-toolkit
4. config-linter (extension)

### Phase 4 - Polish (Month 6+)
1. Web UI for tools
2. Integrations (Slack, PagerDuty, Jira)
3. Kubernetes operator versions
4. SaaS version with multi-tenancy

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
