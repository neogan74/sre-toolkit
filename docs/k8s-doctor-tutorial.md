# k8s-doctor Tutorial

## Introduction

`k8s-doctor` is a comprehensive Kubernetes cluster diagnostics tool that helps you quickly identify health issues, detect common problems, and understand the overall state of your cluster.

## Prerequisites

- Access to a Kubernetes cluster
- Valid kubeconfig file (typically at `~/.kube/config`)
- k8s-doctor binary installed

## Installation

### From Source

```bash
cd sre-toolkit
make build
# Binary will be in bin/k8s-doctor
```

### Install to PATH

```bash
make install
# Installs to $GOPATH/bin
```

## Basic Usage

### Health Check

The `healthcheck` command performs comprehensive health checks on your cluster:

```bash
# Check cluster health with default settings
k8s-doctor healthcheck

# Check specific namespace
k8s-doctor healthcheck -n production

# Output as JSON
k8s-doctor healthcheck -o json

# Use custom kubeconfig
k8s-doctor healthcheck --kubeconfig ~/.kube/staging-config
```

#### What Does Health Check Do?

1. **Node Health**: Checks status of all nodes
   - Ready/NotReady status
   - Resource pressure (Memory, Disk, PID)
   - Cordon status
   - Kubelet version

2. **Pod Health**: Analyzes pod status across the cluster
   - Total pod count by phase (Running, Pending, Failed, etc.)
   - Problem pod detection (CrashLoopBackOff, ImagePullBackOff, etc.)
   - High restart counts

3. **Component Health**: Validates control plane components
   - API server
   - Controller manager
   - Scheduler
   - etcd
   - CoreDNS
   - kube-proxy

#### Example Output

**Table Format (default):**

```
2025-12-26T21:00:00+05:00 INF Connected to cluster version=v1.28.0
2025-12-26T21:00:01+05:00 INF Nodes checked count=3

=== Node Health ===
NODE           STATUS      ROLES           VERSION   ISSUES
----           ------      -----           -------   ------
control-plane  ✓ Ready     control-plane   v1.28.0   0
worker-1       ✓ Ready     worker          v1.28.0   0
worker-2       ✗ NotReady  worker          v1.28.0   1 ⚠

=== Pod Summary ===
Total Pods:     127
Running:        120
Pending:        5
Failed:         2
Succeeded:      0
Unknown:        0

=== Problem Pods (7) ===
NAMESPACE   POD                    STATUS    REASON               RESTARTS
---------   ---                    ------    ------               --------
default     nginx-abc123           Pending   ImagePullBackOff     0
kube-sys    coredns-xyz789         Running   HighRestartCount(12) 12
```

**JSON Format:**

```bash
k8s-doctor healthcheck -o json > cluster-health.json
```

Returns structured JSON data for programmatic analysis.

### Diagnostics

The `diagnostics` command performs deep analysis and categorizes issues by severity:

```bash
# Run diagnostics on entire cluster
k8s-doctor diagnostics

# Focus on specific namespace
k8s-doctor diagnostics -n production

# Get JSON output for CI/CD integration
k8s-doctor diagnostics -o json
```

#### What Does Diagnostics Do?

Analyzes cluster health and categorizes issues:

- **Critical**: Immediate action required (NotReady nodes, CrashLoopBackOff pods, unhealthy components)
- **Warning**: Should be addressed soon (resource pressure, frequent restarts)
- **Info**: For awareness (cordoned nodes)

#### Example Output

```
=== Diagnostics Summary ===
Total Issues:   15
Critical:       3
Warning:        10
Info:           2

=== Node Issues (2) ===
NODE       SEVERITY       TYPE             MESSAGE
----       --------       ----             -------
worker-2   🔴 Critical    NodeNotReady     Node is not in Ready state
worker-1   ⚠️  Warning    NodePressure     Memory pressure detected

=== Pod Issues (12) ===
NAMESPACE    POD                SEVERITY       TYPE                RESTARTS
---------    ---                --------       ----                --------
production   api-deployment     🔴 Critical    CrashLoopBackOff    15
default      nginx-abc          🔴 Critical    ImagePullError      0
monitoring   prometheus         ⚠️  Warning    FrequentRestarts    7

=== System Issues (1) ===
COMPONENT   SEVERITY       TYPE                  MESSAGE
---------   --------       ----                  -------
etcd        🔴 Critical    ComponentUnhealthy    Pod running but not ready
```

#### Exit Codes

The `diagnostics` command uses exit codes for CI/CD integration:

- `0`: Success (no critical issues)
- `1`: Failure (critical issues found)

### Advanced Usage

#### Custom Timeout

For large clusters, you may need longer timeouts:

```bash
k8s-doctor healthcheck --timeout 60s
```

#### Namespace Filtering

Check specific namespaces:

```bash
# Single namespace
k8s-doctor healthcheck -n kube-system

# All namespaces (default)
k8s-doctor healthcheck
```

#### Verbose Output

Get detailed logging:

```bash
k8s-doctor healthcheck --verbose
```

#### Different Kubeconfig Contexts

```bash
# Use specific kubeconfig file
k8s-doctor healthcheck --kubeconfig ~/.kube/staging

# Or set KUBECONFIG environment variable
export KUBECONFIG=~/.kube/staging
k8s-doctor healthcheck
```

## Use Cases

### 1. Pre-Deployment Checks

Run diagnostics before deploying:

```bash
#!/bin/bash
k8s-doctor diagnostics -n production
if [ $? -eq 0 ]; then
    echo "Cluster healthy, proceeding with deployment"
    kubectl apply -f deployment.yaml
else
    echo "Critical issues found, aborting deployment"
    exit 1
fi
```

### 2. Scheduled Health Monitoring

Add to cron for periodic checks:

```bash
# Every hour health check with JSON output
0 * * * * /usr/local/bin/k8s-doctor healthcheck -o json > /var/log/k8s-health-$(date +\%Y\%m\%d-\%H).json
```

### 3. CI/CD Pipeline Integration

```yaml
# GitLab CI example
cluster_health_check:
  stage: pre-deploy
  script:
    - k8s-doctor diagnostics -n production -o json > health-report.json
    - k8s-doctor diagnostics -n production
  artifacts:
    when: always
    paths:
      - health-report.json
  allow_failure: false
```

### 4. Incident Response

Quick cluster overview during incidents:

```bash
# Get comprehensive view
k8s-doctor healthcheck -v

# Focus on problem areas
k8s-doctor diagnostics -n production -o json | jq '.Summary'
```

### 5. Cluster Comparison

Compare health across environments:

```bash
# Production
k8s-doctor healthcheck --kubeconfig ~/.kube/prod -o json > prod-health.json

# Staging
k8s-doctor healthcheck --kubeconfig ~/.kube/staging -o json > staging-health.json

# Compare
diff <(jq . prod-health.json) <(jq . staging-health.json)
```

## Troubleshooting

### Connection Issues

**Problem**: `Failed to connect to cluster`

**Solutions**:
```bash
# Verify kubeconfig
kubectl cluster-info

# Check connectivity
kubectl get nodes

# Specify correct kubeconfig
k8s-doctor healthcheck --kubeconfig /path/to/config
```

### Permission Errors

**Problem**: `Failed to list nodes/pods`

**Solution**: Ensure your user/service account has appropriate RBAC permissions:

```yaml
apiVersion: rbac.authorization.k8s.io/v1
kind: ClusterRole
metadata:
  name: k8s-doctor-reader
rules:
- apiGroups: [""]
  resources: ["nodes", "pods", "componentstatuses"]
  verbs: ["get", "list"]
- apiGroups: [""]
  resources: ["namespaces"]
  verbs: ["get", "list"]
```

### Timeout Issues

**Problem**: `context deadline exceeded`

**Solution**: Increase timeout for large clusters:

```bash
k8s-doctor healthcheck --timeout 120s
```

## Output Format Reference

### Table Format

- Human-readable
- Color-coded status indicators
- Best for terminal viewing
- Default format

### JSON Format

- Machine-parseable
- Complete data export
- Suitable for:
  - Programmatic analysis
  - CI/CD integration
  - Long-term storage
  - Custom reporting tools

Example JSON structure:

```json
{
  "nodes": [
    {
      "name": "worker-1",
      "status": "Ready",
      "roles": ["worker"],
      "version": "v1.28.0",
      "issues": []
    }
  ],
  "pods": {
    "total": 127,
    "running": 120,
    "pending": 5,
    "problemPods": [...]
  },
  "components": [...]
}
```

## Best Practices

1. **Regular Monitoring**: Run health checks regularly, not just during incidents
2. **Baseline Establishment**: Create baseline reports for comparison
3. **Namespace Scoping**: Use `-n` flag to focus on specific namespaces
4. **JSON for Automation**: Use JSON output for automated analysis
5. **Combine with Other Tools**: Use alongside kubectl, prometheus, grafana
6. **CI/CD Integration**: Gate deployments on diagnostics results
7. **Version Tracking**: Keep health check outputs with git commits

## Next Steps

- Try the `audit` command (coming soon) for security checks
- Integrate with monitoring systems
- Create custom dashboards from JSON output
- Set up alerting based on diagnostics

## Support

For issues, feature requests, or questions:
- GitHub Issues: https://github.com/neogan/sre-toolkit/issues
- Documentation: See README.md and plan.md
