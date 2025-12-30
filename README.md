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
Currently working on **webhook ingestion with Kafka**.

# Notes
- [Postgres Time Series Partitioning Notes](./Postgres%20Time%20Series%20Partitioning%20Notes.md) - Query optimization with pg_partman
- [Claude Code Workflow Notes](./Claude%20Code%20Workflow%20Notes.md) - How I use Claude Code to accelerate learning

# Roadmap

Detailed plan: **[docs/roadmap.md](./docs/roadmap.md)**

| Step | Feature | Status |
|------|---------|--------|
| - | Time-series Partitioning (pg_partman) | Done |
| 1 | Webhooks ingestion with Kafka | In Progress |
| 2 | Outbox pattern → CDC → Analytics | Planned |
| 3 | Sharding experiments | Planned |
| 4 | Analytics & Observability | Planned |
| 5 | Infrastructure (K8s, API Gateway) | Planned |
| 5.5 | Simple Deployment Profile | Planned |
| 6 | Deployment | Planned |
| 7 | Simple Frontend (HTMX) | Planned |