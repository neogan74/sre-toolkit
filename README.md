# SRE Toolkit

> A comprehensive collection of production-ready tools for Site Reliability Engineers

[![CI](https://github.com/neogan/sre-toolkit/actions/workflows/ci.yml/badge.svg)](https://github.com/neogan/sre-toolkit/actions/workflows/ci.yml)
[![Go Report Card](https://goreportcard.com/badge/github.com/neogan/sre-toolkit)](https://goreportcard.com/report/github.com/neogan/sre-toolkit)
[![License: MIT](https://img.shields.io/badge/License-MIT-blue.svg)](LICENSE)

## Status

**Stage:** active development, 5 of 7 tools available, the other two in progress · **Last commit:** 2026-09-05 · **Updated:** 2026-10-10

- **Done:** `k8s-doctor`, `alert-analyzer`, `chaos-load`, `config-linter`, `cert-monitor` (inventory reporting by issuer, certificate chain validation), CI and metrics foundation.
- **In progress:** `cert-monitor` Kubernetes secrets scanning (uncommitted edits in `internal/cert-monitor/k8ssecrets`).
- **Early code, not released:** `log-parser` and `db-toolkit` (marked "Coming Soon" below). Proposed next: `slo-gen`, `cost-optimizer`, `incident-cli`. See [plan.md](plan.md) and [docs/tasks/backlog.md](docs/tasks/backlog.md).

## Overview

SRE Toolkit is a collection of powerful command-line utilities designed to help Site Reliability Engineers diagnose, monitor, and improve infrastructure reliability. Built with Go for performance and production readiness.

### What Makes This Different?

- **Production-Ready**: Comprehensive error handling, logging, metrics, and observability
- **Performance-Focused**: Efficient concurrent processing, minimal resource usage
- **Developer-Friendly**: Clean CLI interface, detailed help, progress indicators
- **Security-First**: Built-in security scanning, best practices validation
- **Well-Tested**: High test coverage, integration tests, CI/CD automation

## Tools

### 🏥 k8s-doctor - Kubernetes Health Checker (✅ Available)

Comprehensive Kubernetes cluster diagnostics and health checking.

**Features:**
- Cluster health checks (nodes, pods, components)
- Issue diagnostics (CrashLoopBackOff, resource pressure, etc.)
- Security and best practices audit
- Multiple output formats (table, JSON, HTML reports)

**Quick Start:**
```bash
# Run health checks
k8s-doctor healthcheck

# Run diagnostics
k8s-doctor diagnostics

# Security audit
k8s-doctor audit
```

Landing pages hub: [docs/landing/index.html](docs/landing/index.html)

### 📊 alert-analyzer - Prometheus Alert Optimizer (✅ Available)

Analyze Prometheus/Alertmanager alerts to reduce noise and improve signal.

**Features:**
- Prometheus API integration for alert history collection
- Alertmanager API integration for active alerts
- Frequency analysis of firing alerts
- Identification of noisy, flapping, and correlated alerts
- Temporal pattern analysis by hour of day and weekday
- Actionable recommendations for noisy, unstable, dead, and duplicated alert paths
- Continuous `monitor` mode for Prometheus/Grafana dashboards
- Support for custom lookback periods and resolutions
- Multiple output formats (table, JSON, Markdown)

**Quick Start:**
```bash
# Analyze last 7 days of alerts
alert-analyzer analyze --prometheus-url http://prometheus:9090

# Analyze with custom lookback and top-N results
alert-analyzer analyze --prometheus-url http://prometheus:9090 --lookback 30d --top-n 10

# Include flapping and correlation insights
alert-analyzer analyze --prometheus-url http://prometheus:9090 --show-flapping --show-correlation

# Inspect time-of-day and day-of-week patterns
alert-analyzer analyze --prometheus-url http://prometheus:9090 --show-temporal-patterns

# Generate actionable recommendations
alert-analyzer analyze --prometheus-url http://prometheus:9090 --show-recommendations

# Export a Markdown report
alert-analyzer analyze --prometheus-url http://prometheus:9090 --output markdown

# Notify Slack/email about high-priority problematic rules
alert-analyzer analyze --prometheus-url http://prometheus:9090 \
  --slack-webhook "$SLACK_WEBHOOK_URL" --notify-on high

# Expose continuous analysis metrics for Grafana
alert-analyzer monitor \
  --prometheus-url http://prometheus:9090 \
  --show-flapping \
  --show-correlation \
  --show-temporal-patterns \
  --show-recommendations \
  --interval 1m \
  --metrics-address :8080
```

VictoriaMetrics compatibility stack:
`deployments/docker/alert-analyzer-victoriametrics`

### 💥 chaos-load - Load & Chaos Testing (✅ Available)

Combined load testing and chaos engineering toolkit.

**Features:**
- HTTP load generator with keep-alive support
- Bearer and Basic authentication for protected endpoints
- Configurable concurrency and duration
- Real-time statistics (RPS, Latency percentiles)
- Detailed reporting

**Quick Start:**
```bash
# Run HTTP load test
chaos-load http --url https://example.com --duration 30s --concurrency 20

# Run authenticated load test
chaos-load http --url https://api.example.com --bearer-token "$API_TOKEN" --duration 30s --concurrency 20
```

### ✅ config-linter - Configuration Validator (✅ Available)

Multi-format configuration linter with security checks for Kubernetes YAML, Helm charts, Dockerfiles, and Terraform.

**Features:**
- Kubernetes YAML security audit (privileged containers, missing resource limits, probe checks)
- Dockerfile best practices (pinned tags, non-root USER, no sudo, COPY vs ADD)
- Terraform security checks (open ingress, hardcoded credentials, encryption, local backend)
- Helm chart validation (semver, maintainers, template syntax, plaintext secrets in values)
- Multiple output formats (table, JSON)
- Severity filtering (`--min-severity High` for CI gates)

**Quick Start:**
```bash
# Lint current directory (all supported formats)
config-linter lint

# Lint a specific path
config-linter lint --path ./k8s/

# CI: fail only on High+ issues
config-linter lint --path . --min-severity High

# JSON output for tooling integration
config-linter lint --path . --output json
```

### 🔒 cert-monitor - Certificate Monitoring (✅ Available)

Proactive SSL/TLS certificate expiry monitoring with Kubernetes integration and webhook alerting.

**Features:**
- TLS certificate scanning for hosts, host:port pairs, and HTTPS URLs
- Configurable warning/critical thresholds (days before expiry)
- Kubernetes TLS secret scanning (`kubernetes.io/tls`) across namespaces
- Continuous `watch` mode with configurable scan interval
- Alerting via webhook, Slack and email (SMTP) with `--min-severity` filtering; `watch` dedups repeat alerts
- Prometheus metrics for Grafana dashboards
- Multiple output formats (table, JSON)
- Concurrent scanning for large host lists

**Quick Start:**
```bash
# Scan a single host
cert-monitor scan github.com

# Scan multiple hosts with custom thresholds
cert-monitor scan github.com api.example.com --warn-days 60 --crit-days 14

# JSON output for tooling integration
cert-monitor --output json scan github.com

# Scan Kubernetes TLS secrets in a namespace
cert-monitor k8s --namespace production

# Continuous monitoring every hour with Prometheus metrics
cert-monitor watch --interval 1h --metrics-addr :9101 github.com api.example.com

# CI: exit non-zero if any cert is critical/expired
cert-monitor scan github.com api.example.com

# Alert to Slack (URL from env) and email, only for critical or worse
export SLACK_WEBHOOK_URL=https://hooks.slack.com/services/... SMTP_PASSWORD=...
cert-monitor watch --interval 6h --min-severity critical --renotify-interval 24h \
  --smtp-host smtp.example.com --smtp-user alerts --smtp-from certs@example.com \
  --smtp-to oncall@example.com github.com api.example.com
```

Alerting flags (shared by `scan`, `k8s`, `watch`): `--webhook`, `--slack-webhook`, `--slack-channel`,
`--smtp-host/--smtp-port/--smtp-user/--smtp-from/--smtp-to/--smtp-tls` (`none|starttls|tls`),
`--min-severity` (`warning|critical|expired`), `--alert-chain` (also alert on chain verification failures).
`watch` adds `--renotify-interval` (default 24h; 0 = only on status change). Secrets: `SLACK_WEBHOOK_URL`, `SMTP_PASSWORD`.

Escalation (`watch` only): `--escalate-after 4h` re-alerts **once** when a certificate has been alerting that long,
to `--escalate-slack-webhook` ($`ESCALATE_SLACK_WEBHOOK_URL`) and/or `--escalate-smtp-to` (same `--smtp-*` server).
State is in-memory: a restart restarts the timer. See [docs/cert-monitor-alerting.md](docs/cert-monitor-alerting.md).

Renewal history: add `--history-file certs.json` (or `$CERT_MONITOR_HISTORY`) to `scan`/`k8s`/`watch`, then:

```bash
cert-monitor history --history-file certs.json                       # per-target summary
cert-monitor history --history-file certs.json --events --type renewed --since 720h -o json
```

### 📝 log-parser - Intelligent Log Analyzer (🚧 Coming Soon)

Smart log parsing with pattern detection and anomaly detection.

### 🗄️ db-toolkit - Database Operations Helper (🚧 Coming Soon)

Database health monitoring and automation toolkit.

## Installation

### From Source

```bash
# Clone the repository
git clone https://github.com/neogan/sre-toolkit.git
cd sre-toolkit

# Build all tools
make build-all

# Or build specific tool
make build

# Install to $GOPATH/bin
make install
```

### Using Go Install

```bash
go install github.com/neogan/sre-toolkit/cmd/k8s-doctor@latest
```

## Usage

### k8s-doctor

```bash
# Get help
k8s-doctor --help

# Run health checks
k8s-doctor healthcheck

# Run with verbose output
k8s-doctor healthcheck --verbose

# Export results as JSON
k8s-doctor healthcheck --output json
```

### Configuration

Create a configuration file at `$HOME/.sre-toolkit.yaml`:

```yaml
# Logging configuration
logging:
  level: info        # debug, info, warn, error
  format: console    # console or json
  timeFormat: RFC3339

# Metrics configuration
metrics:
  enabled: true
  address: ":9090"
  path: "/metrics"
```

## Development

### Prerequisites

- Go 1.24 or higher
- Make
- golangci-lint (for linting)

### Setup

```bash
# Install dependencies
make deps

# Run tests
make test

# Run linter
make lint

# Format code
make fmt

# Run all checks
make check
```

### Project Structure

```
sre-toolkit/
├── cmd/              # CLI entry points
│   ├── k8s-doctor/
│   ├── alert-analyzer/
│   └── ...
├── pkg/              # Shared libraries
│   ├── cli/          # CLI framework
│   ├── logging/      # Logging utilities
│   ├── metrics/      # Prometheus metrics
│   └── config/       # Configuration management
├── internal/         # Tool-specific logic
├── docs/             # Documentation
│   ├── backlog.md    # Product backlog
│   ├── architecture.md # System architecture
│   └── plan.md       # Master plan
└── Makefile          # Build automation
```

### Available Make Targets

```bash
make help          # Show all available targets
make build         # Build k8s-doctor
make build-all     # Build all tools
make test          # Run tests
make test-coverage # Run tests with coverage report
make lint          # Run golangci-lint
make fmt           # Format code
make clean         # Clean build artifacts
make run           # Build and run k8s-doctor
make check         # Run all checks
```

## Observability

### Metrics

When metrics are enabled, Prometheus metrics are exposed on `:9090/metrics`:

```bash
# Enable metrics
k8s-doctor healthcheck --metrics-enabled

# Access metrics
curl http://localhost:9090/metrics
```

**Available Metrics:**
- `sre_toolkit_command_executions_total` - Command executions by status
- `sre_toolkit_command_duration_seconds` - Command execution duration
- `sre_toolkit_resources_processed_total` - Resources processed by type
- `sre_toolkit_errors_total` - Errors by command and type

### Logging

Structured logging with zerolog:

```bash
# Console format (default)
k8s-doctor healthcheck

# JSON format
k8s-doctor healthcheck --log-format=json

# Debug level
k8s-doctor healthcheck --log-level=debug
```

## Roadmap

See [plan.md](plan.md) for complete roadmap, [docs/tasks/backlog.md](docs/tasks/backlog.md) for features, and [docs/architecture.md](docs/architecture.md) for system design.

### Done

- [x] Foundation: project structure, CLI framework (Cobra + Viper), logging, metrics, CI/CD
- [x] k8s-doctor: health checks, diagnostics, audit, HTML report
- [x] alert-analyzer: frequency, flapping, correlation, temporal patterns, recommendations, Grafana dashboard, Slack/email/webhook notifications
- [x] chaos-load: HTTP load generation, Kubernetes chaos scenarios, terminal dashboard
- [x] config-linter: Kubernetes, Terraform, Dockerfile, Helm
- [x] cert-monitor: scanning, chain/revocation checks, inventory, Slack/email alerts, escalation, renewal history

### In progress / next

- [ ] log-parser and db-toolkit: first release (see backlog for open items)
- [ ] chaos-load: resource exhaustion and storage chaos; Prometheus export
- [ ] alert-analyzer: Jira integration; notifications from `monitor`
- [ ] New tools: slo-gen, cost-optimizer, incident-cli

## License

MIT License - see [LICENSE](LICENSE) file for details.

## Author

Created by [@neogan](https://github.com/neogan)

---

**Note**: This project is under active development. Some tools are not yet implemented (marked with 🚧).
