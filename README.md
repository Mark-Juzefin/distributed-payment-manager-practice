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

Building **Observability** infrastructure — Prometheus metrics, Grafana dashboards, and distributed tracing. This is a prerequisite for meaningful benchmarks in the paused Inter-Service Communication feature. Added **Security Foundations** track to the roadmap to align with miltech job requirements (TLS, secrets management, least privilege, AuthN/AuthZ).

# Roadmap

Detailed plan: **[docs/roadmap.md](./docs/roadmap.md)**

**Done:**
- **Time-series Partitioning** — pg_partman for dispute_events, query I/O reduced from 200MB to 30MB
- **Kafka Ingestion** — async webhook processing, DLQ, retry with backoff, topic partitioning
- **Ingest Service** — extracted lightweight HTTP→Kafka gateway as separate microservice

**In Progress:**
- **Observability** — Prometheus metrics, Grafana dashboards, correlation IDs, distributed tracing

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