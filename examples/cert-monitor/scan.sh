#!/usr/bin/env bash
# Examples of cert-monitor usage

CERT_MONITOR=${CERT_MONITOR:-cert-monitor}

# Scan a single host
echo "=== Single host scan ==="
$CERT_MONITOR scan github.com

# Scan multiple hosts with custom thresholds
echo ""
echo "=== Multi-host scan with custom thresholds ==="
$CERT_MONITOR scan github.com google.com --warn-days 60 --crit-days 14

# Scan from hosts file
echo ""
echo "=== Scan from hosts file ==="
$CERT_MONITOR scan $(grep -v '^#' hosts.txt | tr '\n' ' ')

# JSON output for tooling integration
echo ""
echo "=== JSON output ==="
$CERT_MONITOR --output json scan github.com

# Scan with Prometheus metrics exposed
echo ""
echo "=== With Prometheus metrics (Ctrl+C to stop) ==="
$CERT_MONITOR --metrics-addr :9101 scan github.com google.com

# Watch mode — continuous monitoring every hour with webhook alerts
echo ""
echo "=== Watch mode (Ctrl+C to stop) ==="
$CERT_MONITOR watch --interval 1h \
  --webhook "${SLACK_WEBHOOK_URL:-}" \
  --warn-days 30 --crit-days 7 \
  github.com google.com
