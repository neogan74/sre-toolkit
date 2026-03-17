# SRE Toolkit

> A comprehensive collection of production-ready tools for Site Reliability Engineers

[![CI](https://github.com/neogan/sre-toolkit/actions/workflows/ci.yml/badge.svg)](https://github.com/neogan/sre-toolkit/actions/workflows/ci.yml)
[![Go Report Card](https://goreportcard.com/badge/github.com/neogan/sre-toolkit)](https://goreportcard.com/report/github.com/neogan/sre-toolkit)
[![License: MIT](https://img.shields.io/badge/License-MIT-blue.svg)](LICENSE)

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
- Support for custom lookback periods and resolutions
- Multiple output formats (table, JSON)

**Quick Start:**
```bash
# Analyze last 7 days of alerts
alert-analyzer analyze --prometheus-url http://prometheus:9090

# Analyze with custom lookback and top-N results
alert-analyzer analyze --prometheus-url http://prometheus:9090 --lookback 30d --top-n 10

# Include flapping and correlation insights
alert-analyzer analyze --prometheus-url http://prometheus:9090 --show-flapping --show-correlation
```

### 💥 chaos-load - Load & Chaos Testing (✅ Available)

Combined load testing and chaos engineering toolkit.

**Features:**
- HTTP load generator with keep-alive support
- Configurable concurrency and duration
- Real-time statistics (RPS, Latency percentiles)
- Detailed reporting

**Quick Start:**
```bash
# Run HTTP load test
chaos-load http --url https://example.com --duration 30s --concurrency 20
```

### ✅ config-linter - Configuration Validator (🚧 Coming Soon)

Multi-format configuration linter with security checks.

### 🔒 cert-monitor - Certificate Monitoring (🚧 Coming Soon)

Proactive SSL/TLS certificate monitoring and alerting.

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

See [plan.md](plan.md) for complete roadmap, [docs/backlog.md](docs/backlog.md) for features, and [docs/architecture.md](docs/architecture.md) for system design.

### Current Phase: Foundation ✅

- [x] Project structure and build system
- [x] CLI framework (Cobra + Viper)
- [x] Logging (zerolog)
- [x] Metrics (Prometheus)
- [x] CI/CD pipeline (GitHub Actions)
- [ ] k8s-doctor MVP implementation

### Next Phase: k8s-doctor MVP (In Progress)

- [ ] Kubernetes client setup
- [ ] Health check implementation
- [ ] Diagnostics engine
- [ ] Best practices audit

## License

MIT License - see [LICENSE](LICENSE) file for details.

## Author

Created by [@neogan](https://github.com/neogan)

---

**Note**: This project is under active development. Some tools are not yet implemented (marked with 🚧).
