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

Building **Outbox → CDC → Analytics** pipeline — reliable event publishing via outbox pattern, custom Go CDC worker (PostgreSQL logical replication), and OpenSearch projection consumer. Core pipeline is working end-to-end; next up is partitioning for the events table and exactly-once semantics exploration.

# Roadmap

Detailed plan with checkpoints: **[docs/roadmap.md](./docs/roadmap.md)**

# Notes
- [Postgres Time Series Partitioning Notes](./Postgres%20Time%20Series%20Partitioning%20Notes.md) - Query optimization with pg_partman
- [Claude Code Workflow Notes](./Claude%20Code%20Workflow%20Notes.md) - How I use Claude Code to accelerate learning

# Architecture

```mermaid
flowchart LR
  EXT["Payment Provider"]
    -->|webhooks| ING["Ingest<br/>:3001"]

  ING -->|Kafka| API["API<br/>:3000"]

  subgraph TX["PostgreSQL (single TX)"]
    direction TB
    BIZ["orders / disputes"]
    EVT["events (outbox)"]
  end
  API --> TX

  EVT -->|WAL logical replication| CDC["CDC Worker"]
  CDC -->|produce| TOPIC["Kafka<br/>domain.events"]
  TOPIC -->|consume| AN["Analytics<br/>Consumer"]
  AN -->|index| OS["OpenSearch"]

  API -.->|capture, representment| SG["Silvergate<br/>(provider API)"]
```

| Service | Path | Description |
|---------|------|-------------|
| **API** :3000 | `cmd/api` | Core business logic, DB owner, Kafka consumers, manual ops |
| **Ingest** :3001 | `cmd/ingest` | HTTP → Kafka gateway for webhooks, no DB |
| **CDC Worker** | `cmd/cdc` | PG WAL → Kafka `domain.events` via logical replication |
| **Analytics** | `cmd/analytics` | Kafka → OpenSearch `domain-events` projection |

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

# Load Testing

Generate realistic data by sending webhook sequences through the full Ingest → API flow.

```bash
# Start services first
make run-dev

# Run load test (default: 10 VUs, 30s)
make loadtest

# Custom parameters
go run ./loadtest -vus 50 -duration 2m -dispute-ratio 0.5
```

Each virtual user creates orders (full lifecycle: created → updated → success/failed) and disputes (30% of successful orders). After running, the database will contain orders, disputes, and their event histories.

# Monitoring

Services expose Prometheus metrics at `/metrics`; Grafana dashboards visualize HTTP latency, error rates, and Kafka throughput.

```bash
make start-monitoring  # Prometheus :9090, Grafana :3100 (admin/admin)
```