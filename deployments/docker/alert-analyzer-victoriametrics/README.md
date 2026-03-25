# Alert-Analyzer VictoriaMetrics Compatibility Stack

This Docker Compose stack is used to validate `alert-analyzer` against a VictoriaMetrics-based setup instead of Prometheus.

## Components

- **VictoriaMetrics** (`:8428`) - stores time series and serves Prometheus-compatible query APIs
- **vmagent** - scrapes sample metrics and remote-writes them into VictoriaMetrics
- **vmalert** (`:8880`) - evaluates alerting rules and remote-writes `ALERTS{}` series back into VictoriaMetrics
- **Node Exporter** (`:9101`) - sample metric source

## Why This Stack Exists

`alert-analyzer` relies on two things:

- `ALERTS{}` history over the query API
- `/api/v1/rules` for dead-rule recommendations

In VictoriaMetrics deployments those concerns are split:

- `ALERTS{}` lives in VictoriaMetrics after `vmalert` remote-writes alert state
- rules are exposed by `vmalert`

This stack configures VictoriaMetrics with `-vmalert.proxyURL=http://vmalert:8880`, so `alert-analyzer` can keep using a single `--prometheus-url http://localhost:8428`.

## Quick Start

```bash
cd deployments/docker/alert-analyzer-victoriametrics
docker compose up -d
```

Wait 3-5 minutes for `vmagent` to populate metrics and for `vmalert` to evaluate rules.

## Manual Validation

From the repo root:

```bash
make build-all

./bin/alert-analyzer analyze \
  --prometheus-url http://localhost:8428 \
  --lookback 1h \
  --show-flapping \
  --show-correlation \
  --show-temporal-patterns \
  --show-recommendations
```

Expected:

- analysis completes without Prometheus-specific API errors
- `frequency_analysis` is populated
- `recommendations` includes `dead_rule` for `TestAlertNeverFiring`

## Smoke Test Script

Run the included smoke test:

```bash
sh ./smoke-test.sh
```

You can override the binary path if needed:

```bash
ALERT_ANALYZER_BIN=../../../bin/alert-analyzer sh ./smoke-test.sh
```

## Useful Checks

Query alert history:

```bash
curl 'http://localhost:8428/api/v1/query?query=ALERTS{}'
```

Check proxied rules endpoint:

```bash
curl 'http://localhost:8428/api/v1/rules'
```

Check `vmalert` directly:

```bash
curl 'http://localhost:8880/api/v1/rules'
```

## Stop the Stack

```bash
docker compose down
```

To remove all data:

```bash
docker compose down -v
```
