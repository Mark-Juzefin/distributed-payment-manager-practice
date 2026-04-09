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
  - [x] Ingest Service extraction: separate microservice (`services/ingest/`), service-based monorepo
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

- **Step 3: Outbox Pattern → CDC → Analytics** ← *paused* — [details](features/003-outbox-cdc-analytics/)
  - [x] Unified `events` table (outbox) — atomic writes in same TX as business data
  - [x] Custom Go CDC worker — PG logical replication (WAL → Kafka `domain.events`)
  - [x] Analytics consumer — Kafka → OpenSearch projection (`domain-events` index)
  - [ ] E2E & integration test revision for outbox/CDC/analytics pipeline
  - [ ] Partitioning for unified events table (pg_partman)
  - [ ] Exactly-once semantics: demonstrate the tradeoffs and limitations
  - [ ] Old event tables cleanup (Strangler Fig completion)

- **Step 4: Inbox Pattern: Reliable Webhook Ingestion** ← *paused* — [details](features/004-inbox-pattern/)
  - [x] Shared Kernel refactoring: decouple Ingest from API domain types
  - [x] Inbox table + Ingest writes: durable webhook storage, return 202 OK
  - [x] DB-queue worker (SKIP LOCKED): poll inbox, forward to API via HTTP, retry logic
  - [ ] Inbox e2e & integration tests: full flow webhook→inbox→worker→API→DB
  - [ ] Concurrent workers with SKIP LOCKED: run N workers claiming from inbox in parallel, observe no double-processing under load
  - [ ] CDC + Kafka variant: inbox + outbox in one TX, CDC publishes to Kafka
  - [ ] Benchmarks & comparison: loadtest both approaches, latency/throughput metrics

- **Step 5: PostgreSQL HA & DR** ← *paused* — [details](features/005-postgres-ha/)

  - [x] Streaming replication: primary + async standby via Docker Compose
  - [x] Read replica routing: HAProxy rw/ro split, app-level routing at repository level
  - [x] Failover/switchover: Patroni + etcd automated failover, HAProxy REST API health checks
  - [x] Monitoring: replication lag, HAProxy metrics, postgres-exporter, Grafana dashboard
  - [ ] Backup/restore: pg_basebackup for PITR, restore verification
  - [ ] Replication lag consistency test: demonstrate read-after-write issues

- **Step 6: Payment System Logic** ← *active* — [details](features/007-payment-system-logic/)

- **Step 7: Sharding Experiments**
  - Infra approach: Citus, Patroni Operator on K8s, or app-level with duplicated docker-compose — decide before implementation
  - [ ] Split orders/disputes across multiple Postgres shards by hash(user_id)
  - [ ] Routing strategies, rebalancing, cross-shard queries, failure modes

- **Step 8: Simple Deployment Profile + VPS Hosting**
  - [ ] Single-node deployment without Kafka dependency (sync mode as default)
  - [ ] HTMX admin dashboard for viewing orders, disputes, events
  - [ ] VPS deployment: systemd services, nginx reverse proxy, basic security hardening
  - [ ] Local dev tooling: research alternatives to goreman (Overmind, process-compose, etc.)

- **Step 9: Infrastructure (Kubernetes, API Gateway, Service Mesh)**
  - [ ] Kubernetes: deploy services, HPA, liveness/readiness, ConfigMaps/Secrets
  - [ ] API Gateway: ingress (NGINX/Traefik/Kong), routing, rate limiting, authn/z
  - [ ] Service mesh: circuit breakers, retries/timeouts (Envoy/Istio-lite)
  - [ ] Postgres access: PgBouncer per service, connection limits
  - [ ] CI/CD: build pipelines, image tagging, per-env configs

- **Step 10: Subscription Engine with Temporal** — [details](features/006-subscription-engine/)
  - [ ] Temporal dev server setup, new `cmd/subscriptions` service skeleton
  - [ ] Subscription CRUD API, PostgreSQL schema, domain model (lifecycle state machine)
  - [ ] Solidgate payment provider mock (Wiremock), tokenized recurring charges
  - [ ] BillingCycleWorkflow — invoice creation, payment charge, webhook reconciliation
  - [ ] SubscriptionWorkflow — lifecycle orchestration, billing loop, signals (cancel/pause/resume)
  - [ ] Dunning & retry logic — exponential retry on decline, past_due state
  - [ ] Payment method update, invoice history API
  - [ ] Integration tests with Temporal test framework, E2E with Wiremock
  - [ ] Observability — Temporal metrics, Grafana dashboard

- **Step 11: Security Foundations**
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
