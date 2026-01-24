# Roadmap

Детальний план фіч для практики highload/distributed systems.

## Completed

- [x] **Time-series Partitioning** - PostgreSQL pg_partman для dispute_events
  - Див: [Postgres Time Series Partitioning Notes](../Postgres%20Time%20Series%20Partitioning%20Notes.md)

- [x] **Step 1: Webhooks ingestion with Kafka**
  - Async webhook processing via Kafka topics
  - Sync/Kafka mode switch via WEBHOOK_MODE env variable
  - Consumer resilience: retry with exponential backoff, panic recovery, DLQ
  - Topic partitioning
  - Details: [features/001-kafka-ingestion/](features/001-kafka-ingestion/) | Notes: [notes.md](features/001-kafka-ingestion/notes.md)

- [x] **Ingest Service Extraction**
  - Extracted Ingest service as a separate microservice
  - Kafka mode: Ingest → Kafka → API consumer
  - Separate binaries: `cmd/ingest/` + `cmd/api/`
  - Service-based monorepo architecture (`internal/api/`, `internal/ingest/`)
  - Details: [features/002-ingest-service-extraction/](features/002-ingest-service-extraction/)

---

## In Progress

### Observability
- **Metrics**: Prometheus instrumentation, key SLIs (webhook latency p50/p95/p99, Kafka lag, error rates)
- **Dashboards**: Grafana dashboards for services health, throughput, latency
- **Tracing**: OpenTelemetry integration, distributed tracing (Jaeger)
- **Profiling**: pprof endpoints for dev, continuous profiling basics
- **SLO thinking**: define target latencies, alerting on violations
- Practice: metrics design, Prometheus/Grafana, distributed tracing, SLO-based reliability
- Details: [features/004-observability/](features/004-observability/)

---

## Paused

### Inter-Service Communication
- Sync mode communication between Ingest and API services
- Progressive approach: HTTP → HTTP + Protobuf → gRPC
- Benchmarking different approaches (Kafka vs HTTP vs gRPC)
- Practice: Protocol Buffers, gRPC, service-to-service communication
- Details: [features/003-inter-service-communication/](features/003-inter-service-communication/)
- **Paused reason:** Need observability first for meaningful benchmarks

---

## Planned

### Step 3: Simple Deployment Profile + VPS Hosting
- Single-node deployment without Kafka dependency (sync mode as default)
- HTMX admin dashboard for viewing orders, disputes, events
- Minimal infrastructure: single PostgreSQL instance
- **VPS deployment**: deploy to a cheap VPS, systemd services, nginx reverse proxy, basic security hardening
- Practice: feature flags, multi-profile configuration, HTMX/SSR, Linux server administration

### Step 4: Outbox Pattern → CDC → Analytics
- Implement outbox tables for reliable event publishing
- Use Debezium/CDC to stream events into OpenSearch or ClickHouse
- Exactly-once semantics: demonstrate the tradeoffs and limitations
- Practice: event-driven consistency, CDC pipelines, projections, analytical indexing

### Step 5: PostgreSQL Replication
- **Streaming replication**: primary-standby setup, synchronous vs asynchronous
- **Read replica routing**: write → primary, read → replica (pgpool or application-level)
- **Failover/switchover**: manual and automated (Patroni basics)
- **Monitoring**: replication lag metrics, alerting on lag thresholds
- Practice: HA patterns, read scaling, failover procedures

### Step 6: Sharding Experiments
- Split orders/disputes across multiple Postgres shards by hash(user_id)
- **Prerequisites**: observability + replication knowledge
- Practice: routing strategies, rebalancing, cross-shard queries, failure modes

### Step 7: Infrastructure (Kubernetes, API Gateway, Service Mesh)
- **Kubernetes**: deploy services, HPA, liveness/readiness, ConfigMaps/Secrets
- **API Gateway**: ingress (NGINX/Traefik/Kong), routing, rate limiting, authn/z
- **Service mesh**: circuit breakers, retries/timeouts (Envoy/Istio-lite)
- **Workflow orchestration**: Temporal for long-running transactions (saga pattern)
- **Postgres access**: PgBouncer per service, connection limits
- **CI/CD**: build pipelines, image tagging, per-env configs
- Practice: service boundaries, platform primitives, reliability patterns

### Experiment — Second Language Module (Rust/C++)
- Separate microservice: Go calls Rust/C++ over gRPC
- Library: Rust crate → shared library + FFI into Go (cgo)
- WASM plugin: rules/logic compiled to wasm, executed by Go
- Implementation idea: Fee & Pricing Engine

---

## Architecture Decision Records (ADRs)

ADRs document significant architectural decisions with context and tradeoffs.

| ADR | Topic | Status |
|-----|-------|--------|
| ADR-001 | Kafka Architecture & Abstractions | Planned (after 003) |
| ADR-002 | Sync vs Async Webhook Processing | Planned (after 003) |

Location: `docs/adr/`