# Payment Manager

**Payment Manager** is a backend service that simulates a merchant system integrated with external payment providers.  
It receives and processes webhook events related to orders and disputes.

## Tech Stack
Go, PostgreSQL, pg_partman, Docker, MongoDB, OpenSearch, Kafka, testcontainers-go, Citus, WireMock

# Goals

- Practice database scaling: time-series partitioning, sharding, replication, and queues
- Experiment with multiple database systems
- Write high-quality Golang code
- Include thorough testing: unit, integration, and end-to-end tests
- Practice metrics collection and benchmarking configurations
- Extensibility in the architecture of application and infrastructure
- Host simplified version on VPS


# Not Goals

- A rational or production-ready domain design 
- Solving real-world fintech problems
- Achieving real performance gains from scaling (metrics and benchmarks here are for learning, not for driving design decisions)

# Status

Starting **Inter-Service Communication** benchmarks — comparing HTTP JSON vs Protobuf vs gRPC for sync mode between Ingest and API services. Observability (Prometheus + Grafana) is ready for measuring latency and throughput differences.

# Roadmap

Detailed plan: **[docs/roadmap.md](./docs/roadmap.md)**

**Done:**
- **Time-series Partitioning** — pg_partman for dispute_events, query I/O reduced from 200MB to 30MB
- **Kafka Ingestion** — async webhook processing, DLQ, retry with backoff, topic partitioning
- **Ingest Service** — extracted lightweight HTTP→Kafka gateway as separate microservice
- **Observability** — Prometheus metrics, Grafana dashboards, correlation IDs, health checks

**In Progress:**
- **Inter-Service Communication** — HTTP vs Protobuf vs gRPC benchmarking

**Planned:**
- **Security Foundations** — TLS, secrets management, Postgres roles, AuthN/AuthZ
- **VPS Deployment** — single-node prod profile, HTMX admin, nginx, systemd
- **Outbox + CDC** — reliable event publishing, Debezium, exactly-once semantics
- **PostgreSQL HA & DR** — streaming replication, failover, backup/restore
- **Sharding** — horizontal partitioning by user_id
- **K8s + Service Mesh** — HPA, ingress, circuit breakers, Temporal workflows

# Notes
- [Postgres Time Series Partitioning Notes](./Postgres%20Time%20Series%20Partitioning%20Notes.md) - Query optimization with pg_partman
- [Claude Code Workflow Notes](./Claude%20Code%20Workflow%20Notes.md) - How I use Claude Code to accelerate learning

# Services Architecture

The system consists of two services:

## API Service (cmd/api)
- **Purpose**: Core business logic, database owner, manual operations
- **Responsibilities**:
  - Manual operations: capture, hold
  - Read endpoints: GET /orders, /disputes, /events
  - Webhook processing (sync mode only)
  - Kafka consumers (kafka mode)
  - Database migrations
- **Port**: 3000

## Ingest Service (cmd/ingest)
- **Purpose**: Lightweight HTTP → Kafka gateway
- **Responsibilities**:
  - Accepts webhooks from payment providers
  - Publishes to Kafka topics
  - No database, no business logic
- **Port**: 3001

# Domain Entities

## Order
Payment order from external provider.

| Field | Description |
|-------|-------------|
| `order_id` | Provider's order identifier |
| `user_id` | Customer UUID |
| `status` | `created` → `updated` → `success` / `failed` |
| `on_hold` | Manual hold flag |
| `hold_reason` | `manual_review` or `risk` |

**Events:** `webhook_received`, `hold_set`, `hold_cleared`, `capture_requested`, `capture_completed`, `capture_failed`

## Dispute
Chargeback/dispute linked to an order.

| Field | Description |
|-------|-------------|
| `dispute_id` | Internal dispute identifier |
| `order_id` | FK to related order |
| `status` | `open` → `under_review` → `submitted` → `won` / `lost` / `closed` / `canceled` |
| `reason` | Dispute reason from provider |
| `amount`, `currency` | Disputed amount |
| `evidence_due_at` | Deadline for evidence submission |

**Events:** `webhook_opened`, `webhook_updated`, `provider_decision`, `evidence_submitted`, `evidence_added`

## State Machines

```
Order:   created ──→ updated ──→ success
                           └──→ failed

Dispute: open ──→ under_review ──→ submitted ──→ won
                                          └──→ lost
                                          └──→ closed
                       └──→ canceled
```

# Monitoring

Services expose Prometheus metrics at `/metrics`; Grafana dashboards visualize HTTP latency, error rates, and Kafka throughput.

```bash
make start-monitoring  # Prometheus :9090, Grafana :3100 (admin/admin)
```