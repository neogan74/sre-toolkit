# alert-analyzer Tutorial

## Introduction

`alert-analyzer` is a Prometheus alert analysis tool that helps you identify noisy alerts, reduce alert fatigue, and optimize your alerting rules. It analyzes alert history to find patterns, calculate firing frequencies, and provide actionable insights for improving your monitoring effectiveness.

## Prerequisites

- Access to a Prometheus server
- Prometheus with alert history (ALERTS{} metric available)
- Optional: Access to an Alertmanager server (for active alerts)
- alert-analyzer binary installed

## Installation

### From Source

```bash
cd sre-toolkit
make build
# Binary will be in bin/alert-analyzer
```

### Install to PATH

```bash
make install
# Installs to $GOPATH/bin
```

## Basic Usage

### Analyzing Alert History

The `analyze` command connects to Prometheus and analyzes alert patterns:

```bash
# Analyze last 7 days (default)
alert-analyzer analyze --prometheus-url http://localhost:9090

# Analyze last 30 days
alert-analyzer analyze --prometheus-url http://prom:9090 --lookback 30d

# Analyze last 24 hours with 1-minute resolution
alert-analyzer analyze --prometheus-url http://prom:9090 --lookback 24h --resolution 1m

# Analyze including current active alerts from Alertmanager
alert-analyzer analyze \
  --prometheus-url http://prom:9090 \
  --alertmanager-url http://alertmanager:9093

# Show top 20 noisiest alerts
alert-analyzer analyze --prometheus-url http://prom:9090 --top-n 20

# Output as JSON for automation
alert-analyzer analyze --prometheus-url http://prom:9090 --output json
```

### Command Flags

| Flag | Description | Default |
|------|-------------|---------|
| `--prometheus-url` | Prometheus server URL (required) | - |
| `--lookback` | Time range to analyze (e.g., 7d, 24h, 30d) | `7d` |
| `--resolution` | Query resolution (e.g., 1m, 5m, 15m) | `5m` |
| `--output, -o` | Output format: table or json | `table` |
| `--top-n` | Number of top alerts to show | `20` |
| `--alertmanager-url` | Alertmanager server URL (optional) | - |
| `--timeout` | Request timeout | `30s` |
| `--insecure` | Skip TLS verification | `false` |

## What Does Alert Analyzer Do?

### 1. Alert Collection
- Connects to Prometheus API
- Queries `ALERTS{}` metric over specified time range
- Extracts alert metadata (name, labels, state, timestamps)
- Groups alerts by name and instance
- (Optional) Connects to Alertmanager to fetch currently active alerts

### 2. Frequency Analysis
- Calculates total firing count per alert
- Tracks unique alert instances
- Measures alert duration
- Identifies most frequent alerts

### 3. Summary Statistics
- Total number of alert firings
- Unique alert count
- Time range analyzed
- Top N noisiest alerts

## Example Output

### Table Format (Default)

```
2025-12-26T21:00:00+05:00 INF Starting alert-analyzer
2025-12-26T21:00:00+05:00 INF Connected to Prometheus url=http://localhost:9090
2025-12-26T21:00:05+05:00 INF Alert data collected total_alerts=1523 unique_alerts=45

=== Alert Analysis Summary ===
Time Range:     2025-12-19 21:00:00 → 2025-12-26 21:00:00 (7 days)
Total Firings:  1,523
Unique Alerts:  45

=== Top 20 Noisiest Alerts ===
ALERT NAME                    FIRINGS    SEVERITY    NAMESPACE       DESCRIPTION
----------                    -------    --------    ---------       -----------
HighMemoryUsage              342        warning     production      Memory > 80%
DatabaseConnectionFlap       298        critical    production      DB connection unstable
CPUHighUsage                 187        warning     staging         CPU > 75%
HighSystemLoad              156        warning     production      Load avg high
PodRestartingFrequently     123        warning     default         Pod restarting
DiskSpaceRunningLow          89        info        production      Disk > 85%
APILatencyHigh               67        warning     production      API p95 > 500ms
...
```

### JSON Format

```bash
$ alert-analyzer analyze --prometheus-url http://localhost:9090 -o json | jq '.'
```

```json
{
  "summary": {
    "total_firings": 1523,
    "unique_alerts": 45,
    "time_range_start": "2025-12-19T21:00:00Z",
    "time_range_end": "2025-12-26T21:00:00Z"
  },
  "top_alerts": [
    {
      "name": "HighMemoryUsage",
      "firing_count": 342,
      "severity": "warning",
      "namespace": "production",
      "percentage": 22.4
    },
    {
      "name": "DatabaseConnectionFlap",
      "firing_count": 298,
      "severity": "critical",
      "namespace": "production",
      "percentage": 19.6
    }
  ]
}
```

## Advanced Usage

### Using with Victoria Metrics

alert-analyzer is compatible with Victoria Metrics (requires vmalert component):

```bash
alert-analyzer analyze --prometheus-url http://victoriametrics:8428
```

### Custom TLS Configuration

```bash
# Skip TLS verification (development only)
alert-analyzer analyze --prometheus-url https://prom:9090 --insecure

# Use custom timeout
alert-analyzer analyze --prometheus-url http://prom:9090 --timeout 60s
```

### Filtering and Analysis

```bash
# Analyze short time range for recent issues
alert-analyzer analyze --prometheus-url http://prom:9090 --lookback 1h

# High resolution for detailed analysis
alert-analyzer analyze --prometheus-url http://prom:9090 --resolution 30s --lookback 6h

# Focus on top 10 worst offenders
alert-analyzer analyze --prometheus-url http://prom:9090 --top-n 10
```

## Use Cases

### 1. Reducing Alert Fatigue

**Problem:** Too many alerts, important ones get lost

**Solution:**
```bash
# Identify noisiest alerts over 30 days
alert-analyzer analyze --prometheus-url http://prom:9090 --lookback 30d --top-n 20
```

**Actions:**
- Increase `for:` duration for top noisy alerts
- Adjust thresholds to reduce false positives
- Disable or tune alerts firing hundreds of times

### 2. Weekly Alert Health Check

**Problem:** Need regular monitoring of alerting effectiveness

**Solution:**
```bash
# Weekly cron job
0 9 * * 1 alert-analyzer analyze \
  --prometheus-url http://prom:9090 \
  --lookback 7d \
  --output json > /tmp/alerts-$(date +%Y%m%d).json
```

**Actions:**
- Review trends week-over-week
- Identify new noisy alerts
- Track alert reduction progress

### 3. Post-Incident Analysis

**Problem:** Determine which alerts fired during incident

**Solution:**
```bash
# Analyze specific time window
alert-analyzer analyze \
  --prometheus-url http://prom:9090 \
  --lookback 2h \
  --resolution 1m
```

**Actions:**
- Identify alert storm patterns
- Find alerts that should have fired but didn't
- Improve alert correlation

### 4. CI/CD Integration

**Problem:** Prevent alert rule changes that increase noise

**Solution:**
```bash
# Before deploying new alert rules
alert-analyzer analyze --prometheus-url http://staging-prom:9090 \
  --lookback 24h \
  --output json > before.json

# After deploying
alert-analyzer analyze --prometheus-url http://staging-prom:9090 \
  --lookback 24h \
  --output json > after.json

# Compare (custom script)
./compare-alert-noise.sh before.json after.json
```

### 5. Multi-Cluster Comparison

**Problem:** Compare alert noise across environments

**Solution:**
```bash
# Production
alert-analyzer analyze --prometheus-url http://prod-prom:9090 \
  --output json > prod-alerts.json

# Staging
alert-analyzer analyze --prometheus-url http://staging-prom:9090 \
  --output json > staging-alerts.json

# Compare noise levels
jq '.summary.total_firings' prod-alerts.json
jq '.summary.total_firings' staging-alerts.json
```

## Development Environment

### Local Testing with Docker Compose

The repository includes a complete Docker Compose environment for testing:

```bash
cd deployments/docker/alert-analyzer
docker-compose up -d
```

**Services:**
- **Prometheus** (port 9090) - Metrics and alerting
- **Node Exporter** (port 9100) - Sample metrics
- **Grafana** (port 3000) - Visualization

**Sample Alerts Included:**
- HighMemoryUsage (noisy)
- DatabaseConnectionFlap (flapping)
- CPUHighUsage (noisy)
- APIServerDown (critical)
- TestAlertNeverFiring (dead rule)

### Testing Workflow

```bash
# 1. Start environment
docker-compose up -d

# 2. Wait for alerts to fire (5-10 minutes)

# 3. Run analysis
alert-analyzer analyze --prometheus-url http://localhost:9090

# 4. Test JSON output
alert-analyzer analyze --prometheus-url http://localhost:9090 -o json | jq '.summary'

# 5. Stop environment
docker-compose down
```

See `deployments/docker/alert-analyzer/README.md` for complete setup guide.

## Output Format Reference

### Table Output Fields

| Column | Description |
|--------|-------------|
| ALERT NAME | Prometheus alert rule name |
| FIRINGS | Total number of times alert fired |
| SEVERITY | Label value (critical/warning/info) |
| NAMESPACE | Kubernetes namespace (if applicable) |
| DESCRIPTION | Alert annotation or summary |

### JSON Output Structure

```json
{
  "summary": {
    "total_firings": int,      // Total alert instances
    "unique_alerts": int,      // Number of distinct alerts
    "time_range_start": string,
    "time_range_end": string
  },
  "top_alerts": [
    {
      "name": string,          // Alert name
      "firing_count": int,     // Times fired
      "severity": string,      // Label value
      "namespace": string,     // Label value
      "percentage": float      // % of total firings
    }
  ]
}
```

## Troubleshooting

### Connection Issues

**Problem:** `Failed to connect to Prometheus`

**Solutions:**
```bash
# Verify Prometheus is accessible
curl http://localhost:9090/api/v1/status/config

# Check network/firewall
telnet localhost 9090

# Use correct URL scheme
alert-analyzer analyze --prometheus-url http://prom:9090  # not https

# Increase timeout
alert-analyzer analyze --prometheus-url http://prom:9090 --timeout 60s
```

### No Alerts Found

**Problem:** `Alert data collected total_alerts=0`

**Possible Causes:**
1. No alerts fired in time range
2. ALERTS{} metric not available
3. Prometheus recording rules disabled

**Solutions:**
```bash
# Check if ALERTS{} exists
curl 'http://localhost:9090/api/v1/query?query=ALERTS{}'

# Reduce lookback period
alert-analyzer analyze --prometheus-url http://prom:9090 --lookback 1h

# Verify alert rules are configured
curl http://localhost:9090/api/v1/rules
```

### High Memory Usage

**Problem:** analyzer consumes too much memory

**Solutions:**
```bash
# Reduce time range
alert-analyzer analyze --prometheus-url http://prom:9090 --lookback 7d

# Increase resolution (fewer data points)
alert-analyzer analyze --prometheus-url http://prom:9090 --resolution 15m

# Process in batches (manual approach)
alert-analyzer analyze --lookback 7d  # Week 1
alert-analyzer analyze --lookback 7d  # Week 2
```

### Victoria Metrics Compatibility

**Problem:** Using VictoriaMetrics but no alerts found

**Solution:**
```bash
# Ensure vmalert is running and configured
curl http://victoriametrics:8428/api/v1/query?query=ALERTS{}

# Point to correct VictoriaMetrics URL
alert-analyzer analyze --prometheus-url http://victoriametrics:8428
```

## Best Practices

### 1. Regular Analysis Schedule

Run weekly or bi-weekly to track trends:

```bash
# Weekly Monday morning report
0 9 * * 1 /usr/local/bin/alert-analyzer analyze \
  --prometheus-url http://prom:9090 \
  --lookback 7d \
  --output json | mail -s "Weekly Alert Report" sre-team@company.com
```

### 2. Start with Long Lookback

Get comprehensive view before optimizing:

```bash
# 30 days gives good statistical sample
alert-analyzer analyze --prometheus-url http://prom:9090 --lookback 30d
```

### 3. Focus on Top Offenders

Don't try to fix everything at once:

```bash
# Focus on top 5 worst alerts
alert-analyzer analyze --prometheus-url http://prom:9090 --top-n 5
```

### 4. Store Historical Data

Track progress over time:

```bash
# Monthly snapshots
alert-analyzer analyze --prometheus-url http://prom:9090 \
  --output json > alerts-$(date +%Y-%m).json
```

### 5. Combine with Other Tools

Integrate with existing workflows:

```bash
# Export to CSV for spreadsheet analysis
alert-analyzer analyze --prometheus-url http://prom:9090 -o json | \
  jq -r '.top_alerts[] | [.name, .firing_count, .severity] | @csv'
```

## Integration Examples

### Slack Notifications

```bash
#!/bin/bash
# Send weekly alert report to Slack

REPORT=$(alert-analyzer analyze \
  --prometheus-url http://prom:9090 \
  --lookback 7d \
  --top-n 5 \
  --output json)

TOP_ALERT=$(echo "$REPORT" | jq -r '.top_alerts[0].name')
FIRING_COUNT=$(echo "$REPORT" | jq -r '.top_alerts[0].firing_count')

curl -X POST $SLACK_WEBHOOK \
  -H 'Content-Type: application/json' \
  -d "{
    \"text\": \"📊 Weekly Alert Report\",
    \"attachments\": [{
      \"color\": \"warning\",
      \"fields\": [{
        \"title\": \"Noisiest Alert\",
        \"value\": \"$TOP_ALERT fired $FIRING_COUNT times\",
        \"short\": false
      }]
    }]
  }"
```

### Grafana Annotation

```bash
#!/bin/bash
# Create Grafana annotation when noise threshold exceeded

TOTAL=$(alert-analyzer analyze \
  --prometheus-url http://prom:9090 \
  --lookback 1d \
  -o json | jq '.summary.total_firings')

if [ "$TOTAL" -gt 1000 ]; then
  curl -X POST http://grafana:3000/api/annotations \
    -H "Authorization: Bearer $GRAFANA_API_KEY" \
    -H "Content-Type: application/json" \
    -d "{
      \"text\": \"Alert storm detected: $TOTAL firings in 24h\",
      \"tags\": [\"alert-analyzer\", \"alert-storm\"]
    }"
fi
```

### Jira Ticket Creation

```bash
#!/bin/bash
# Create Jira ticket for noisy alerts

alert-analyzer analyze \
  --prometheus-url http://prom:9090 \
  --lookback 7d \
  --output json | \
jq -r '.top_alerts[] | select(.firing_count > 100)' | \
while read -r alert; do
  NAME=$(echo "$alert" | jq -r '.name')
  COUNT=$(echo "$alert" | jq -r '.firing_count')

  curl -X POST $JIRA_API_URL/issue \
    -u $JIRA_USER:$JIRA_TOKEN \
    -H "Content-Type: application/json" \
    -d "{
      \"fields\": {
        \"project\": {\"key\": \"SRE\"},
        \"summary\": \"Reduce noise for alert: $NAME\",
        \"description\": \"Alert fired $COUNT times in 7 days\",
        \"issuetype\": {\"name\": \"Task\"}
      }
    }"
done
```

## Next Steps

After analyzing your alerts:

1. **Tune Alert Rules**
   - Increase `for:` duration for noisy alerts
   - Adjust thresholds based on actual patterns
   - Add `annotations` for better context

2. **Implement Alert Routing**
   - Route low-priority alerts to different channels
   - Use Alertmanager `routes` and `matchers`
   - Create severity-based escalation

3. **Track Progress**
   - Re-run analysis weekly
   - Measure noise reduction
   - Document improvements

4. **Advanced Analysis** (Coming Soon)
   - Flapping detection
   - Alert correlation
   - Temporal patterns
   - Recommendations engine

## Version Information

Current version: 0.1.0
Features: Frequency analysis, basic reporting

See project roadmap for upcoming features:
- Flapping alert detection
- Alert correlation analysis
- Temporal pattern recognition
- Automated recommendations
- Grafana dashboard integration

## Additional Resources

- **Repository:** https://github.com/neogan/sre-toolkit
- **Issues:** Report bugs and feature requests
- **Documentation:** `/docs` directory
- **Examples:** `/deployments/docker/alert-analyzer`

## Contributing

Found a bug or have a feature request? Please open an issue!

Want to contribute? See `CONTRIBUTING.md` (coming soon).

---

**Happy Alert Analyzing!** 📊
