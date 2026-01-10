# SRE Toolkit - Architecture

This document describes the high-level architecture of the tools in the SRE Toolkit.

## High Level Overview

All tools share a common CLI framework based on Cobra and Viper, using a shared internal library structure for consistency.

```mermaid
graph TD
    User[User] --> CLI[CLI Framework (Cobra)]
    CLI --> Config[Config (Viper)]
    CLI --> Logger[Logger (Zerolog)]
    
    subgraph Shared[Shared Packages]
        Config
        Logger
        Metrics[Prometheus Metrics]
    end
    
    CLI --> ToolLogic[Tool Specific Logic]
    ToolLogic --> Shared
```

## Tools Architecture

### 1. k8s-doctor

The `k8s-doctor` tool is designed to diagnose Kubernetes cluster issues by running a series of health checks and diagnostics.

```mermaid
flowchart TD
    CMD[Command: healthcheck/diagnostics] --> Client[K8s Client Wrapper]
    
    subgraph Checks [Health Checks]
        Nodes[Node Check]
        Pods[Pod Check]
        Components[Component Check]
        Events[Event Analysis]
    end
    
    Client --> Checks
    Checks --> Diagnostics[Diagnostics Engine]
    
    subgraph Analysis
        Diagnostics --> Rules[Rule Engine]
        Rules --> Severity[Severity Classifier]
    end
    
    Severity --> Reporter[Reporter]
    
    Reporter --> JSON[JSON Output]
    Reporter --> Table[Table/Terminal Status]
    
    K8s((Kubernetes API))
    Client <--> K8s
```

### 2. alert-analyzer

`alert-analyzer` connects to Prometheus to fetch historical alert data and performs statistical analysis to identify noise.

```mermaid
flowchart TD
    CMD[Command: analyze] --> Collector[Alert Collector]
    
    subgraph DataSources
        Prom{{Prometheus API}}
        AM{{Alertmanager API}}
    end
    
    Collector <--> Prom
    Collector -.-> AM
    
    Collector --> Storage[In-Memory Storage]
    
    subgraph Analysis Engine
        Storage --> Freq[Frequency Analyzer]
        Storage --> Flap[Flapping Detector]
        Storage --> Pattern[Pattern Recognition]
    end
    
    Freq --> Aggregator[Result Aggregator]
    Flap --> Aggregator
    
    Aggregator --> Reporter[Reporter]
    Reporter --> Output[Table/JSON Report]
```

### 3. chaos-load

`chaos-load` generates load while optionally injecting chaos faults. It uses a worker pool pattern for concurrency.

```mermaid
flowchart LR
    CMD[Command] --> Controller[Load Controller]
    
    subgraph WorkerPool
        W1[Worker 1]
        W2[Worker 2]
        Wn[Worker N]
    end
    
    Controller --> WorkerPool
    
    subgraph Stats
        Collector[Stats Collector]
        Metrics[Prometheus Exporter]
    end
    
    WorkerPool -->|Results| Collector
    Collector --> Metrics
    Collector --> UI[Terminal UI / Report]
    
    subgraph Target
        App[Target Application]
    end
    
    WorkerPool -->|HTTP Req| App
    
    Chaos[Chaos Injector] -.->|Faults| App
    Controller -.-> Chaos
```

### 4. slo-gen (Proposed)

Architecture for the proposed SLO generator tool.

```mermaid
flowchart TD
    Input[User Input / Service] --> Analyzer[Metric Analyzer]
    
    subgraph Prom [Prometheus]
        HistData[Historical Metrics]
    end
    
    Analyzer <-->|Query Latency/Errors| Prom
    
    Analyzer --> Calculator[SLO Calculator]
    
    Calculator --> Targets[Suggested Targets]
    
    Targets --> Generator[Resource Generator]
    
    subgraph Output
        TF[Terraform/OpenTofu]
        Grafana[Grafana JSON]
    end
    
    Generator --> TF
    Generator --> Grafana
```

### 5. cost-optimizer (Proposed)

Architecture for the cost optimization tool.

```mermaid
flowchart TD
    Collector[Resource Collector] --> K8s((Kubernetes API))
    Collector --> Cloud((Cloud Provider API))
    
    subgraph Analysis
        Usage[Usage vs Request]
        Idle[Idle Resource Check]
        Orphan[Orphan Resource Check]
    end
    
    Collector --> Usage
    Collector --> Idle
    Collector --> Orphan
    
    Usage --> Recommender[Recommendation Engine]
    Idle --> Recommender
    Orphan --> Recommender
    
    Recommender --> Report[Cost Report]
    Report --> Savings[Potential Savings $$]
```

### 6. config-linter (Coming Soon)

Architecture for the multi-format configuration linter.

```mermaid
flowchart TD
    CMD[Command: lint] --> Parser[File Parser]
    
    subgraph Formats
        YAML[YAML/K8s]
        HCL[Terraform HCL]
        Dockerfile
    end
    
    Parser -->|Parse| YAML
    Parser -->|Parse| HCL
    Parser -->|Parse| Dockerfile
    
    subgraph Validation Engine
        OPA[OPA / Rego Policies]
        Schema[JSON Schema Validator]
        Static[Static Rules]
    end
    
    YAML --> Schema
    YAML --> OPA
    
    HCL --> OPA
    HCL --> Static
    
    Dockerfile --> Static
    Dockerfile --> OPA
    
    OPA --> Results
    Schema --> Results
    Static --> Results
    
    Results --> Reporter[Reporter]
    Reporter --> UI[CLI Output]
```

### 7. cert-monitor (Coming Soon)

Architecture for SSL/TLS certificate monitoring.

```mermaid
flowchart TD
    CMD[Command: scan] --> Scanner[Certificate Scanner]
    
    subgraph Sources
        Endpoint[Web Endpoints]
        K8sSecrets[K8s Secrets]
        Files[Local Files]
    end
    
    Scanner -->|TLS Handshake| Endpoint
    Scanner -->|K8s API| K8sSecrets
    Scanner -->|Read| Files
    
    subgraph Analyzer
        Expiry[Expiration Checker]
        Chain[Chain Validation]
        Revocation[OCSP/CRL Check]
    end
    
    Scanner --> Analyzer
    
    Analyzer --> AlertEngine[Alert Engine]
    
    subgraph Notifications
        Slack
        Metrics[Prometheus Metrics]
        Log[Structured Log]
    end
    
    AlertEngine --> Slack
    AlertEngine --> Metrics
    AlertEngine --> Log
```

### 8. log-parser (Coming Soon)

Architecture for the smart log parser and analyzer.

```mermaid
flowchart TD
    Input[Log Stream/File] --> Parser[Smart Parser]
    
    subgraph Decoders
        JSON
        Logfmt
        Regex
        Syslog
    end
    
    Parser --> Decoders
    Decoders --> StructuredLog[Structured Entry]
    
    subgraph Analysis
        Pattern[Pattern Matcher]
        Anomaly[Anomaly Detector ML]
        Stats[Field Statistics]
    end
    
    StructuredLog --> Analysis
    
    Analysis --> TUI[Terminal UI]
    Analysis --> Exporter[Loki/ES Exporter]
```

### 9. db-toolkit (Coming Soon)

Architecture for the database operations helper.

```mermaid
flowchart TD
    CMD[Command] --> Manager[Connection Manager]
    
    subgraph Databases
        PostgreSQL
        MySQL
    end
    
    Manager --> Databases
    
    subgraph Modules
        Health[Health Checker]
        Backup[Backup/Restore]
        Perf[Performance Analyzer]
    end
    
    Databases --> Health
    Databases --> Backup
    Databases --> Perf
    
    Health --> Report
    Backup --> Storage[S3/GCS]
    Perf --> Insights[Optimization Insights]
```
