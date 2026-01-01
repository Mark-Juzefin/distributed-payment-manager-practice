# Roadmap

Детальний план фіч для практики highload/distributed systems.

## Completed

- [x] **Time-series Partitioning** - PostgreSQL pg_partman для dispute_events
  - Див: [Postgres Time Series Partitioning Notes](../Postgres%20Time%20Series%20Partitioning%20Notes.md)

---

## In Progress

### Step 1: Webhooks ingestion with Kafka
Details: [features/001-kafka-ingestion/](features/001-kafka-ingestion/)

### Architecture Review
Details: [features/002-architecture-review/](features/002-architecture-review/)

---

## Planned

### Step 2: Outbox pattern → CDC → Analytics
- Implement outbox tables per shard.
- Use Debezium/CDC to stream events into OpenSearch or ClickHouse.
- Practice: event-driven consistency, projections, analytical indexing.

### Step 3: Sharding experiments
- Split orders/disputes across multiple Postgres shards by hash(user_id).
- Practice: routing, rebalancing, joins across shards.

### Step 4: Analytics & Observability
- Explore metrics, dashboards, and query analytics in OpenSearch / ClickHouse.
- Benchmark queries across partitions and shards.
- Practice: metrics collection, Grafana dashboards, query optimization.

### Step 5: Infrastructure (Microservices, Kubernetes, API Gateway)
- Break monolith into services (Ingest, Orders, Disputes, Analytics API).
- **Kubernetes**: deploy services, HPA, liveness/readiness, ConfigMaps/Secrets.
- **API Gateway**: ingress (NGINX/Traefik/Kong), routing, rate limiting, authn/z.
- **Service-to-service**: gRPC/HTTP, retries/timeouts, circuit breakers (e.g., Envoy/Istio-lite later).
- **Workflow orchestration**: Temporal for long-running transactions (dispute flows, saga pattern), distributed coordination.
- **Postgres access**: PgBouncer per service, connection limits, migration strategy.
- **CI/CD**: build pipelines, image tagging, per-env configs.
- **Platform ops**: centralized logs, metrics (Prometheus), tracing (OTel/Jaeger).
- Practice: service boundaries, platform primitives, reliability patterns, workflow-driven architecture.

### Step 5.5: Simple Deployment Profile
- Create "simple" profile that works on basic VPS without complex infrastructure.
- **What this includes:**
  - Single PostgreSQL without partitioning (or with minimal partitioning)
  - Synchronous event processing (no Kafka required)
  - Optional: HTMX admin panel for basic operations
- **Note:** This may be a separate, simplified build of the system rather than the full highload version.
- **Goal:** Have a deployable product while continuing highload experiments in parallel.
- Practice: feature flags, dependency injection, multi-profile configuration.

### Step 6: Deployment
- Run the full system on Kubernetes (local k3s or Minikube).
- **OR** deploy Simple Profile to a cheap VPS.
- Experiment with horizontal scaling, liveness/readiness probes.
- Practice: infra basics, container orchestration, ops skills.

### Step 7: Simple Frontend (HTMX)
- Build a lightweight dashboard to view orders, disputes, and events.
- Practice: HTMX, server-side rendering, integrating with APIs.
