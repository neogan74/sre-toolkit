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
- `--concurrency`: Number of concurrent workers (Default: 10)
- `--duration`: Total duration of the test (e.g., 30s, 1m, 5m) (Default: 30s)
- `--requests`: Limit the total number of requests (Optional, 0 for unlimited within duration)

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

## Best Practices

1.  **Start Small**: Begin with low concurrency (e.g., 2-5 workers) to verify connectivity before scaling up.
2.  **Monitor Your Target**: Always monitor server-side metrics (CPU, Memory, DB load) while running tests.
3.  **Check for Rate Limits**: Many public APIs have rate limiting. Testing against them might result in your IP being blocked.
4.  **Use in Non-Production**: Unless you are performing "Chaos Engineering" in a controlled manner, always run load tests in staging or sandbox environments.

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
