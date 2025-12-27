# Roadmap

Детальний план фіч для практики highload/distributed systems.

## Completed

- [x] **Time-series Partitioning** - PostgreSQL pg_partman для dispute_events
  - Див: [Postgres Time Series Partitioning Notes](../Postgres%20Time%20Series%20Partitioning%20Notes.md)

---

## In Progress

### Step 1: Webhooks ingestion with Kafka
Details: [features/001-kafka-ingestion.md](features/001-kafka-ingestion.md)

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
- **Postgres access**: PgBouncer per service, connection limits, migration strategy.
- **CI/CD**: build pipelines, image tagging, per-env configs.
- **Platform ops**: centralized logs, metrics (Prometheus), tracing (OTel/Jaeger).
- Practice: service boundaries, platform primitives, reliability patterns.

### Step 6: Deployment
- Run the system on Kubernetes (local k3s or Minikube).
- Experiment with horizontal scaling, liveness/readiness probes.
- Later, deploy a minimal setup to a cheap VPS.
- Practice: infra basics, container orchestration, ops skills.

### Step 7: Simple Frontend (HTMX)
- Build a lightweight dashboard to view orders, disputes, and events.
- Practice: HTMX, server-side rendering, integrating with APIs.
