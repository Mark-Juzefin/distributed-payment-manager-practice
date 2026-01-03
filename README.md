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
Currently working on **Ingest Service Extraction** (microservices architecture).

# Notes
- [Postgres Time Series Partitioning Notes](./Postgres%20Time%20Series%20Partitioning%20Notes.md) - Query optimization with pg_partman
- [Claude Code Workflow Notes](./Claude%20Code%20Workflow%20Notes.md) - How I use Claude Code to accelerate learning

# Roadmap

Detailed plan: **[docs/roadmap.md](./docs/roadmap.md)**

| Step | Feature | Status |
|------|---------|--------|
| - | Time-series Partitioning (pg_partman) | Done |
| 1 | Webhooks ingestion with Kafka | Done |
| 1.5 | Ingest Service Extraction | In Progress |
| 2 | Outbox pattern → CDC → Analytics | Planned |
| 3 | Sharding experiments | Planned |
| 4 | Analytics & Observability | Planned |
| 5 | Infrastructure (K8s, API Gateway) | Planned |
| 5.5 | Simple Deployment Profile | Planned |
| 6 | Deployment | Planned |
| 7 | Simple Frontend (HTMX) | Planned |

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

# Running Modes

## Sync Mode (default for dev)
Runs API service only with WEBHOOK_MODE=sync. Webhooks are processed directly by API service.

```bash
make run-dev  # Runs API service only with WEBHOOK_MODE=sync
```

**Architecture:**
```
Webhook → API Service (HTTP) → Domain Logic → PostgreSQL
```

## Kafka Mode (production)
Runs both API + Ingest services. Webhooks are routed through Kafka.

```bash
make run-kafka  # Runs both API + Ingest services via goreman
```

**Architecture:**
```
Webhook → Ingest Service (HTTP) → Kafka → API Consumer → Domain Logic → PostgreSQL
```

**Note**: Requires [goreman](https://github.com/mattn/goreman): `go install github.com/mattn/goreman@latest`

## Standalone Services

```bash
make run-api     # Run API service only
make run-ingest  # Run Ingest service only
```

## Docker Compose

```bash
make run  # Runs both services + infrastructure in containers
```