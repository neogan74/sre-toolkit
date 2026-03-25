# Alert-Analyzer Development Environment

This Docker Compose stack provides a local Prometheus environment for testing the alert-analyzer tool.

## Components

- **Prometheus**: Metrics collection and alerting (port 9090)
- **Node Exporter**: Sample application generating metrics (port 9100)
- **Alert Analyzer Monitor**: Periodic analysis exporter (port 8080 inside the compose network)
- **Grafana**: Dashboard visualization with provisioned alert-analyzer dashboard (port 3000)

## Sample Alerts

The stack includes several types of alerts for testing:

1. **HighMemoryUsage** - Noisy alert that fires frequently with short duration
2. **DatabaseConnectionFlap** - Flapping alert that fires/resolves repeatedly
3. **APIServerDown** - Critical alert (won't fire in normal operation)
4. **TestAlertNeverFiring** - Alert that never fires (for testing recommendations)
5. **CPUHighUsage** - Another noisy alert
6. **HighSystemLoad** - Correlated with HighMemoryUsage
7. **LowDiskSpace** - Info-level alert

## Quick Start

### 1. Start the Stack

```bash
cd deployments/docker/alert-analyzer
docker-compose up -d
```

### 2. Verify Prometheus is Running

Open http://localhost:9090 in your browser.

Navigate to Status → Targets to see all targets are UP.

Navigate to Alerts to see the configured alert rules.

### 3. Wait for Data Collection

Wait 5-10 minutes for alerts to start firing and generate some history.

### 4. Open the Dashboard

Open http://localhost:3000 and sign in with `admin/admin`.

Grafana is provisioned with:
- Prometheus datasource
- `Alert Analyzer Overview` dashboard

### 5. Run Alert-Analyzer Manually

From the project root:

```bash
# Build the tool
make build

# Analyze alerts
./bin/alert-analyzer analyze --prometheus-url http://localhost:9090

# Analyze with JSON output
./bin/alert-analyzer analyze --prometheus-url http://localhost:9090 --output json

# Analyze last 1 hour (for quick testing)
./bin/alert-analyzer analyze --prometheus-url http://localhost:9090 --lookback 1h
```

## Accessing Services

- **Prometheus UI**: http://localhost:9090
- **Grafana**: http://localhost:3000 (admin/admin)
- **Alert Analyzer Metrics**: http://localhost:8080/metrics inside compose via `alert-analyzer-monitor:8080`
- **Node Exporter Metrics**: http://localhost:9100/metrics

## Monitor Mode

The stack now includes an `alert-analyzer-monitor` service that runs:

```bash
go run ./cmd/alert-analyzer monitor \
  --prometheus-url http://prometheus:9090 \
  --show-flapping \
  --show-correlation \
  --show-temporal-patterns \
  --show-recommendations \
  --interval 1m \
  --metrics-address :8080
```

Prometheus scrapes this exporter and Grafana reads the resulting metrics.

## Useful Prometheus Queries

Check alert history:
```promql
ALERTS{}
```

Check firing alerts:
```promql
ALERTS{alertstate="firing"}
```

Check specific alert:
```promql
ALERTS{alertname="HighMemoryUsage"}
```

## Stopping the Stack

```bash
docker-compose down
```

To remove all data:
```bash
docker-compose down -v
```

## Customizing Alerts

Edit `alert_rules.yml` to add or modify alerts, then reload Prometheus:

```bash
curl -X POST http://localhost:9090/-/reload
```

Or restart the container:
```bash
docker-compose restart prometheus
```

## Troubleshooting

### No alerts firing

Wait a few minutes for alerts to evaluate. Check the Prometheus UI → Alerts to see their status.

### Cannot connect to Prometheus

Ensure the container is running:
```bash
docker-compose ps
```

Check logs:
```bash
docker-compose logs prometheus
docker-compose logs alert-analyzer-monitor
```

### Alerts resolve too quickly

Adjust the `for:` duration in `alert_rules.yml` to make alerts fire longer.
