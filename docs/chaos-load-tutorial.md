# chaos-load Tutorial

## Introduction

`chaos-load` is a powerful load testing and chaos engineering utility designed for Site Reliability Engineers. It allows you to simulate high-traffic scenarios and verify system resilience by generating concurrent HTTP load and analyzing performance metrics.

## Prerequisites

- Go 1.24 or higher (for building from source)
- Network access to the target URL
- SRE Toolkit repository cloned locally

## Installation

### From Source

Build the `chaos-load` binary specifically:

```bash
cd sre-toolkit
make chaos-load
# Binary will be in bin/chaos-load
```

Or build all tools in the toolkit:

```bash
make build-all
```

### Install to PATH

```bash
make install
# Note: Ensure $GOPATH/bin is in your PATH
```

## Basic Usage

### Running a Simple HTTP Load Test

The `http` command is the primary way to generate load:

```bash
# Run a 30-second test with 10 concurrent workers
./bin/chaos-load http --url https://example.com --duration 30s --concurrency 10
```

### Key Parameters

- `--url`: The target URL to test (Required)
- `--method`: HTTP method for each request (Default: `GET`)
- `--body`: Request payload for `POST`, `PUT`, and similar methods
- `--bearer-token`: Adds `Authorization: Bearer <token>` to every request
- `--basic-username`: Username for HTTP Basic authentication
- `--basic-password`: Password for HTTP Basic authentication
- `--concurrency`: Number of concurrent workers (Default: 10)
- `--duration`: Total duration of the test (e.g., 30s, 1m, 5m) (Default: 30s)
- `--requests`: Limit the total number of requests (Optional, 0 for unlimited within duration)

`Bearer` and `Basic` modes are mutually exclusive. For Basic authentication, `--basic-username` is required and `--basic-password` is optional.

## Understanding Results

After the test completes, `chaos-load` provides a detailed summary:

### Example Output

```text
=== Load Test Results ===
Total Requests: 1052
Total Duration: 5.07s
Requests/sec:   207.49
Errors:         0

Latency:
  p50: 45.2ms
  p95: 82.1ms
  p99: 120.5ms
  Max: 156.2ms

Status Codes:
  [200]: 1052
```

### Metrics Explained

1.  **Requests/sec (RPS)**: The average throughput achieved during the test.
2.  **Errors**: Number of failed requests (e.g., connection timeouts, DNS issues).
3.  **Latency Percentiles**:
    - **p50 (Median)**: 50% of requests were faster than this value.
    - **p95**: 95% of requests were faster than this value. Often used to identify "tail latency" issues.
    - **p99**: 99% of requests were faster than this value. Critical for high-reliability systems.
4.  **Status Codes**: A breakdown of HTTP response codes returned by the server.

## Advanced Examples

### Stress Testing with High Concurrency

To find the breaking point of a service, increase concurrency:

```bash
./bin/chaos-load http --url https://api.myservice.com/v1/health \
    --concurrency 100 \
    --duration 1m
```

### Fixed Workload Testing

Run exactly 10,000 requests as fast as possible:

```bash
./bin/chaos-load http --url https://example.com \
    --requests 10000 \
    --concurrency 50 \
    --duration 10m # Set duration high enough to allow all requests to finish
```

### Bearer Token Authentication

Send authenticated requests to APIs protected by bearer tokens:

```bash
./bin/chaos-load http --url https://api.myservice.com/v1/health \
    --bearer-token "$API_TOKEN" \
    --concurrency 20 \
    --duration 1m
```

### Basic Authentication

Load test endpoints behind HTTP Basic authentication:

```bash
./bin/chaos-load http --url https://staging.example.com/internal/status \
    --basic-username sre \
    --basic-password changeme \
    --concurrency 5 \
    --requests 200
```

## Best Practices

1.  **Start Small**: Begin with low concurrency (e.g., 2-5 workers) to verify connectivity before scaling up.
2.  **Monitor Your Target**: Always monitor server-side metrics (CPU, Memory, DB load) while running tests.
3.  **Use Scoped Credentials**: Prefer short-lived or low-privilege bearer tokens and dedicated Basic credentials for test traffic.
4.  **Check for Rate Limits**: Many public APIs have rate limiting. Testing against them might result in your IP being blocked.
5.  **Use in Non-Production**: Unless you are performing "Chaos Engineering" in a controlled manner, always run load tests in staging or sandbox environments.

## Troubleshooting

### Timeouts or Refused Connections
## Kubernetes Chaos Scenarios

The \`k8s\` subcommand provides chaos engineering capabilities for Kubernetes clusters. These are designed to test how your applications recover from unexpected failures.

### Prerequisites

\`\`\`bash
# Install chaos-load binary
make build-all

# Ensure kubectl is installed and configured
kubectl version
\`\`\`

### Pod Killing

Simulates unexpected pod terminations to test application resilience.

\`\`\`bash
# Kill a random pod with default grace period (30s)
chaos-load k8s pod-kill \\
  --namespace production \\
  --selector app=web \\
  --count 1

# Kill multiple pods sequentially with 5-second interval
chaos-load k8s pod-kill \\
  --namespace production \\
  --selector app=web \\
  --count 5 \\
  --interval 5s

# Force kill (no grace period) - tests abrupt termination
chaos-load k8s pod-kill \\
  --namespace production \\
  --selector app=web \\
  --grace-period 0s \\
  --count 1
\`\`\`

**Parameters:**
- \`--namespace, -n\`: Target namespace (default: \`default\`)
- \`--selector, -l\`: Label selector to match pods (e.g., \`app=web\`, \`env=prod\`)
- \`--grace-period\`: Termination grace period (default: 30s). Use \`0s\` for force kill.
- \`--interval\`: Time between kills when \`--count > 1\` (default: 10s)
- \`--count\`: Number of pods to kill sequentially (default: 1)
- \`--dry-run\`: Print what would be killed without taking action

**What to Expect:**
- Pods matching the selector will be terminated
- Application restarts automatically if managed by Deployment/StatefulSet
- Monitor for application recovery (pods restart, readiness probes pass)

**Use Cases:**
1. **Resilience Testing**: Verify your application recovers gracefully from pod failures
2. **Leader Election**: Test if your HA system correctly elects new leader when pod dies
3. **Connection Pooling**: Confirm connection pools re-establish after disruption
4. **Rollback Validation**: Ensure canaries aren't affected by pod loss

### Node Draining

Safely removes workloads from a node for maintenance or chaos scenarios.

\`\`\`bash
# Cordon and drain a node
chaos-load k8s node-drain \\
  --node worker-1

# Cordon node only (mark unschedulable) without draining
chaos-load k8s node-drain \\
  --node worker-1 \\
  --timeout 0s

# Drain with dry-run to preview what would happen
chaos-load k8s node-drain \\
  --node worker-1 \\
  --dry-run
\`\`\`

**Parameters:**
- \`--node\`: Name of the node to drain (Required)
- \`--grace-period\`: Pod termination grace period (default: 30s)
- \`--timeout\`: Timeout for the entire drain operation (default: 5m)
- \`--ignore-daemonsets\`: Skip DaemonSet-managed pods (default: true)
- \`--delete-emptydir-data\`: Delete local data in emptyDir volumes (default: false)
- \`--dry-run\`: Print what would be drained without taking action

**What Happens During Drain:**
1. **Cordon**: Node is marked \`Unschedulable\` — no new pods can be scheduled
2. **Eviction**: Non-mirror pods are evicted one by one
   - DaemonSet pods are skipped (unless \`--ignore-daemonsets\` is set)
   - Pods with emptyDir volumes are skipped (unless \`--delete-emptydir-data\` is set)
   - Pods in \`kube-system\` are never evicted
3. **Termination**: Each evicted pod is terminated gracefully
4. **Retry**: Controllers try to reschedule pods on other nodes

**Safety Features:**
- **Mirror Pod Detection**: Automatically skips static pods managed by kubelet
- **DaemonSet Protection**: DaemonSet pods protected by default (configurable)
- **EmptyDir Safety**: Local data in emptyDir volumes protected by default

**Use Cases:**
1. **Maintenance Testing**: Simulate node drain during upgrades
2. **Failure Recovery**: Test cluster recovery after node failure
3. **Capacity Planning**: Verify cluster can handle workload without a specific node
4. **Chaos Validation**: Combine with pod killing to test cascading failures

### Testing Workload Resilience

Combine HTTP load testing with Kubernetes chaos for comprehensive resilience testing:

\`\`\`bash
# Terminal 1: Generate load while pods are healthy
chaos-load http --url https://myapp.production \\
  --concurrency 50 \\
  --duration 5m &

# Terminal 2: Kill random pods every 30 seconds
chaos-load k8s pod-kill \\
  --namespace production \\
  --selector app=web \\
  --count 10 \\
  --interval 30s &

# Wait for results and analyze
\`\`\`

**Resilience Checklist:**
- [ ] Application restarts automatically when pods die
- [ ] Service endpoints are updated after pod loss
- [ ] Graceful shutdown completes successfully
- [ ] Connection pools reconnect without errors
- [ ] Database connections are recovered
- [ ] Rate limiting prevents overwhelming remaining instances
## Introduction

`chaos-load` is a powerful load testing and chaos engineering utility designed for Site Reliability Engineers. It allows you to simulate high-traffic scenarios and verify system resilience by generating concurrent HTTP load and analyzing performance metrics.

## Prerequisites

- Go 1.24 or higher (for building from source)
- Network access to the target URL
- SRE Toolkit repository cloned locally

## Installation

### From Source

Build the `chaos-load` binary specifically:

```bash
cd sre-toolkit
make chaos-load
# Binary will be in bin/chaos-load
```

Or build all tools in the toolkit:

```bash
make build-all
```

### Install to PATH

```bash
make install
# Note: Ensure $GOPATH/bin is in your PATH
```

## Basic Usage

### Running a Simple HTTP Load Test

The `http` command is the primary way to generate load:

```bash
# Run a 30-second test with 10 concurrent workers
./bin/chaos-load http --url https://example.com --duration 30s --concurrency 10
```

### Key Parameters

- `--url`: The target URL to test (Required)
- `--method`: HTTP method for each request (Default: `GET`)
- `--body`: Request payload for `POST`, `PUT`, and similar methods
- `--bearer-token`: Adds `Authorization: Bearer <token>` to every request
- `--basic-username`: Username for HTTP Basic authentication
- `--basic-password`: Password for HTTP Basic authentication
- `--concurrency`: Number of concurrent workers (Default: 10)
- `--duration`: Total duration of the test (e.g., 30s, 1m, 5m) (Default: 30s)
- `--requests`: Limit the total number of requests (Optional, 0 for unlimited within duration)

`Bearer` and `Basic` modes are mutually exclusive. For Basic authentication, `--basic-username` is required and `--basic-password` is optional.

## Understanding Results

After the test completes, `chaos-load` provides a detailed summary:

### Example Output

```text
=== Load Test Results ===
Total Requests: 1052
Total Duration: 5.07s
Requests/sec:   207.49
Errors:         0

Latency:
  p50: 45.2ms
  p95: 82.1ms
  p99: 120.5ms
  Max: 156.2ms

Status Codes:
  [200]: 1052
```

### Metrics Explained

1.  **Requests/sec (RPS)**: The average throughput achieved during the test.
2.  **Errors**: Number of failed requests (e.g., connection timeouts, DNS issues).
3.  **Latency Percentiles**:
    - **p50 (Median)**: 50% of requests were faster than this value.
    - **p95**: 95% of requests were faster than this value. Often used to identify "tail latency" issues.
    - **p99**: 99% of requests were faster than this value. Critical for high-reliability systems.
4.  **Status Codes**: A breakdown of HTTP response codes returned by the server.

## Advanced Examples

### Stress Testing with High Concurrency

To find the breaking point of a service, increase concurrency:

```bash
./bin/chaos-load http --url https://api.myservice.com/v1/health \
    --concurrency 100 \
    --duration 1m
```

### Fixed Workload Testing

Run exactly 10,000 requests as fast as possible:

```bash
./bin/chaos-load http --url https://example.com \
    --requests 10000 \
    --concurrency 50 \
    --duration 10m # Set duration high enough to allow all requests to finish
```

### Bearer Token Authentication

Send authenticated requests to APIs protected by bearer tokens:

```bash
./bin/chaos-load http --url https://api.myservice.com/v1/health \
    --bearer-token "$API_TOKEN" \
    --concurrency 20 \
    --duration 1m
```

### Basic Authentication

Load test endpoints behind HTTP Basic authentication:

```bash
./bin/chaos-load http --url https://staging.example.com/internal/status \
    --basic-username sre \
    --basic-password changeme \
    --concurrency 5 \
    --requests 200
```

## Best Practices

1.  **Start Small**: Begin with low concurrency (e.g., 2-5 workers) to verify connectivity before scaling up.
2.  **Monitor Your Target**: Always monitor server-side metrics (CPU, Memory, DB load) while running tests.
3.  **Use Scoped Credentials**: Prefer short-lived or low-privilege bearer tokens and dedicated Basic credentials for test traffic.
4.  **Check for Rate Limits**: Many public APIs have rate limiting. Testing against them might result in your IP being blocked.
5.  **Use in Non-Production**: Unless you are performing "Chaos Engineering" in a controlled manner, always run load tests in staging or sandbox environments.

## Troubleshooting

### Timeouts or Refused Connections

**Problem**: High error count with "connection refused" or "context deadline exceeded".

**Solutions**:
- Verify the target URL is accessible from your machine.
- Check if the server is overwhelmed by the current concurrency level.
- Ensure no firewall or load balancer is dropping connections.

### "Too many open files" Error

**Problem**: Operating system limit on open file descriptors reached.

**Solution**: Increase the `ulimit` for open files in your terminal session:
```bash
ulimit -n 4096
```

## Next Steps

- Explore `k8s-doctor` for cluster-level health analysis alongside your load tests.
- Integrate `chaos-load` into your CI/CD pipelines for performance regression testing.
