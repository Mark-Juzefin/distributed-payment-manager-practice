# Roadmap

Детальний план фіч для практики highload/distributed systems.

## Steps

- [x] **Time-series Partitioning** — PostgreSQL pg_partman для dispute_events
  - Див: [Postgres Time Series Partitioning Notes](../Postgres%20Time%20Series%20Partitioning%20Notes.md)

- **Step 1: Kafka Ingestion Pipeline** — [details](features/001-kafka-ingestion-pipeline/)
  - [x] Async webhook processing via Kafka topics
  - [x] Sync/Kafka mode switch via WEBHOOK_MODE env variable
  - [x] Consumer resilience: retry with exponential backoff, panic recovery, DLQ
  - [x] Topic partitioning
  - [x] Ingest Service extraction: separate microservice (`cmd/ingest/`), service-based monorepo
  - [x] HTTP sync mode: internal endpoints for service-to-service calls
  - [ ] Graceful shutdown for Kafka components (DLQ flush ordering)
  - [ ] Realistic user_id lookup (cache layer, lookup service)

- **Step 2: Observability** — [details](features/002-observability/)
  - [x] Prometheus metrics: HTTP latency histograms, request counters, Kafka processing
  - [x] Grafana dashboards: service health, Kafka throughput
  - [x] Correlation IDs across services
  - [x] Health checks (/health/live, /health/ready)
  - [ ] Kafka consumer lag metric
  - [ ] Distributed tracing: Jaeger/OTLP integration
  - [ ] Profiling: pprof endpoints for CPU/memory profiling
  - [ ] Audit logging: structured audit trail for compliance
  - [ ] Log sampling for high-frequency events

- **Step 3: Outbox Pattern → CDC → Analytics** ← *active* — [details](features/003-outbox-cdc-analytics/)
  - [x] Unified `events` table (outbox) — atomic writes in same TX as business data
  - [x] Custom Go CDC worker — PG logical replication (WAL → Kafka `domain.events`)
  - [x] Analytics consumer — Kafka → OpenSearch projection (`domain-events` index)
  - [ ] Partitioning for unified events table (pg_partman)
  - [ ] Exactly-once semantics: demonstrate the tradeoffs and limitations
  - [ ] Old event tables cleanup (Strangler Fig completion)

- **Step 4: Inbox Pattern: Reliable Webhook Ingestion** — [details](features/004-inbox-pattern/)
  - [ ] Inbox table for durable webhook storage before processing
  - [ ] Store-and-forward: save raw payload → return 200 OK → process async
  - [ ] Backpressure and replay capabilities for incoming webhooks
  - [ ] Shared Kernel refactoring: decouple Ingest from API domain types
  - [ ] Reuse CDC infrastructure from Step 3 for inbox processing

- **Step 5: PostgreSQL HA & DR**
  - [ ] Streaming replication: primary-standby setup, synchronous vs asynchronous
  - [ ] Read replica routing: write → primary, read → replica (pgpool or application-level)
  - [ ] Failover/switchover: manual and automated (Patroni basics)
  - [ ] Backup/restore: pg_dump logical backups, pg_basebackup for PITR, restore verification
  - [ ] Monitoring: replication lag metrics, backup success/failure alerts, RTO/RPO tracking

- **Step 6: Simple Deployment Profile + VPS Hosting**
  - [ ] Single-node deployment without Kafka dependency (sync mode as default)
  - [ ] HTMX admin dashboard for viewing orders, disputes, events
  - [ ] VPS deployment: systemd services, nginx reverse proxy, basic security hardening
  - [ ] Local dev tooling: research alternatives to goreman (Overmind, process-compose, etc.)

- **Step 7: Sharding Experiments**
  - [ ] Split orders/disputes across multiple Postgres shards by hash(user_id)
  - [ ] Routing strategies, rebalancing, cross-shard queries, failure modes

- **Step 8: Infrastructure (Kubernetes, API Gateway, Service Mesh)**
  - [ ] Kubernetes: deploy services, HPA, liveness/readiness, ConfigMaps/Secrets
  - [ ] API Gateway: ingress (NGINX/Traefik/Kong), routing, rate limiting, authn/z
  - [ ] Service mesh: circuit breakers, retries/timeouts (Envoy/Istio-lite)
  - [ ] Workflow orchestration: Temporal for long-running transactions (saga pattern)
  - [ ] Postgres access: PgBouncer per service, connection limits
  - [ ] CI/CD: build pipelines, image tagging, per-env configs

- **Step 9: Security Foundations**
  - [ ] TLS: TLS termination on reverse proxy (nginx/traefik), HTTPS for external endpoints
  - [ ] Secrets management: separate config vs secrets, sops/age or docker secrets
  - [ ] Least privilege: separate Postgres roles (migrations user, app RW, readonly)
  - [ ] AuthN/AuthZ basics: API key or JWT for admin endpoints, HMAC for webhooks
  - [ ] mTLS (optional): internal service-to-service TLS for gRPC

---

## Architecture Decision Records (ADRs)

ADRs document significant architectural decisions with context and tradeoffs.

| ADR | Topic | Status |
|-----|-------|--------|
| ADR-001 | Kafka Architecture & Abstractions | Planned |
| ADR-002 | Sync vs Async Webhook Processing | Planned |

Location: `docs/adr/`
