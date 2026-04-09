# Step 2: Observability

**Status:** Done (core complete, optional subtasks deferred)

## Overview

Observability infrastructure for the microservice architecture: Prometheus metrics, Grafana dashboards, correlation IDs, health checks, structured logging.

**Motivation:**
- Metrics are prerequisite for optimization decisions (JSON vs Protobuf, HTTP vs gRPC)
- Prometheus/Grafana — industry standard
- Distributed tracing is critical for microservice debugging
- SLO-based thinking — foundation of reliability

## Completed

### HTTP Metrics (Subtask 1)
- Prometheus instrumentation (`prometheus/client_golang`)
- `/metrics` endpoint for both services
- HTTP handler latency histogram (p50/p95/p99)
- Request counter by endpoint and status

### Kafka Metrics (Subtask 2)
- Kafka message processing duration histogram
- `WithMetrics` middleware in messaging middleware chain
- Processing counter by topic, consumer group, status

### Health Checks (Subtask 3)
- `/health/live` — liveness (process alive)
- `/health/ready` — readiness (dependencies OK: DB, Kafka)
- Health check handlers for both services

### Correlation IDs (Subtask 4)
- Generate/propagate `X-Correlation-ID` header
- Included in all log entries via slog CorrelationHandler
- Passed through Kafka messages

### Grafana Dashboards (Subtask 5)
- Docker Compose with Prometheus + Grafana (`docker compose --profile monitoring up -d`)
- Service health dashboard (RPS, latency, errors)
- Kafka dashboard (processing throughput)
- Auto-provisioned on Grafana start

### Dev Infrastructure Refactoring (Subtask 8)
- Simplified env file management (`env/common.env`, `endpoints.host.env`, etc.)
- `run-dev` as alias to `run-kafka` (default dev mode)
- Goreman via `go run` (no manual install)
- DLQ topics auto-created in kafka-init

### Logger Refactoring (Subtask 9)
- Migrated from zerolog to Go stdlib slog
- Structured logging API (key-value pairs)
- Automatic source location (file:line)
- Context-first design: `correlation_id` auto-injected via custom slog Handler

---

## Architecture Decision Records

### ADR-1: Prometheus over custom metrics

**Decision:** Use Prometheus client library.

**Rationale:**
- Industry standard, pull-based model (no push gateway needed)
- Rich ecosystem (Grafana, alerting)
- Well-maintained Go client library

### ADR-2: slog over zerolog

**Decision:** Migrate to Go stdlib slog.

**Rationale:**
- No external dependencies
- Structured logging natively (key-value, not printf)
- Custom Handler chain: CorrelationHandler → JSONHandler
- Automatic source location via `AddSource: true`
- Standard library — long-term stability

---

## Deferred Work

### Kafka Consumer Lag Metric
- `Stats()` method added to consumer, but full lag metric not wired yet

### Distributed Tracing (Subtask 6)
- OpenTelemetry SDK integration
- Jaeger for visualization
- Trace propagation through HTTP and Kafka

### Profiling (Subtask 7)
- pprof endpoints (`/debug/pprof/`)
- Basic profiling documentation

### Audit Logging (Subtask 10)
- Separate audit log stream for business operations
- Track: who called what (user/system), what changed, correlation_id
- Key operations: capture, hold, dispute transitions
- Structured format for compliance/forensics

### Logger Improvements
- Log sampling for high-frequency events
- Performance evaluation if needed

---

## Infrastructure

### Monitoring Stack

```
monitoring/
├── prometheus.yml                    # Scrape config (api:3000, ingest:3001)
└── grafana/
    ├── provisioning/
    │   ├── datasources/prometheus.yml
    │   └── dashboards/default.yml
    └── dashboards/
        ├── service-health.json       # HTTP metrics (RPS, latency, errors)
        └── kafka.json                # Kafka processing metrics
```

**Access:**
- Prometheus: http://localhost:9090
- Grafana: http://localhost:3100 (admin/admin)

### Env Files

```
env/
├── common.env           # Ports, Kafka topics
├── endpoints.host.env   # localhost URLs (PG, Kafka, OpenSearch, Silvergate)
├── endpoints.docker.env # docker URLs (for docker-compose)
├── api.env              # API-specific config
└── ingest.env           # Ingest-specific config
```

---

## Key Files

| File | Purpose |
|------|---------|
| `pkg/metrics/registry.go` | Prometheus registry |
| `pkg/metrics/http.go` | HTTP metric definitions |
| `pkg/metrics/kafka.go` | Kafka metric definitions |
| `pkg/metrics/middleware.go` | Gin metrics middleware |
| `pkg/logger/logger.go` | `Setup(Options)`, level parsing |
| `pkg/logger/correlation.go` | `CorrelationHandler` (slog) |
| `pkg/logger/gin.go` | `GinBodyLogger()` middleware |
| `services/api/messaging/middleware.go` | `WithMetrics` Kafka middleware |
| `monitoring/` | Prometheus + Grafana configs and dashboards |
