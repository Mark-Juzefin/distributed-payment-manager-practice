# Payment Manager

**Payment Manager** is a backend service that simulates a merchant system integrated with external payment providers.  
It receives and processes webhook events related to orders and disputes.

## Tech Stack
Go, PostgreSQL, pg_partman, Docker, MongoDB, OpenSearch, Kafka, testcontainers-go, Citus, WireMock

# Goals

- Practice database scaling: time-series partitioning, sharding, replication, and queues.
- Experiment with multiple database systems
- Write high-quality Golang code
- Include thorough testing: unit, integration, and end-to-end tests
- Practice metrics collection and benchmarking configurations. 


# Not Goals

- A rational or production-ready domain design 
- Solving real-world fintech problems
- Achieving real performance gains from scaling (metrics and benchmarks here are for learning, not for driving design decisions)

# Status
Earlier experiments: [Postgres Time Series Partitioning Notes](./Postgres%20Time%20Series%20Partitioning%20Notes.md).

Currently working on **webhook ingestion with Kafka**.

# Roadmap


## Step 1: Webhooks ingestion with Kafka

### Schema

```mermaid
flowchart LR
  %% External → Ingest
  EXT["External Provider<br/>webhooks: orders, disputes"]
    -->|HTTP| ING["Ingest API<br/>(auth, validation)"]

  %% Kafka: key = user_id (узгоджена хеш-функція з шардингом)
  ING -->|"produce(key = user_id)"| TOP_O["Kafka topic: orders.webhooks<br/>partitions P0..Pn"]
  ING -->|"produce(key = user_id)"| TOP_D["Kafka topic: disputes.webhooks<br/>partitions P0..Pm"]
  class TOP_O,TOP_D kafka;

  %% ==== Sharded Postgres (by hash(user_id)) ====
  subgraph SHARDS["Sharded Postgres Cluster (by hash(user_id))"]
    direction LR
    subgraph SHARD_A["PG Shard A • primary"]
      direction TB
      A_ORD["orders<br/>order_events (partitioned)"]
      A_DIS["disputes<br/>dispute_events (partitioned)"]
    end
    subgraph SHARD_B["PG Shard B • primary"]
      direction TB
      B_ORD["orders<br/>order_events (partitioned)"]
      B_DIS["disputes<br/>dispute_events (partitioned)"]
    end
  end

  %% ==== Services & consumers (1 worker per partition) ====
  subgraph ORDER_SVC["Order Service"]
    direction TB
    subgraph CGO["Consumer Group: orders.webhooks"]
      O0["Worker P0"] -->|"writes to shard (P0 % N)"| A_ORD
      O1["Worker P1"] -->|"writes to shard (P1 % N)"| B_ORD
      O2["Worker P2"] -->|"writes to shard (P2 % N)"| A_ORD
    end
  end
  TOP_O --> O0
  TOP_O --> O1
  TOP_O --> O2

  subgraph DISPUTE_SVC["Dispute Service"]
    direction TB
    subgraph CGD["Consumer Group: disputes.webhooks"]
      D0["Worker P0"] -->|"writes to shard (P0 % N)"| A_DIS
      D1["Worker P1"] -->|"writes to shard (P1 % N)"| B_DIS
    end
  end
  TOP_D --> D0
  TOP_D --> D1

  %% DLQ (per topic)
  TOP_O -. on error .-> DLQ_O["orders.webhooks.DLQ"]
  TOP_D -. on error .-> DLQ_D["disputes.webhooks.DLQ"]

  classDef kafka fill:#f2f9ff,stroke:#7aa7ff,color:#1c3d7a,stroke-width:1px;

```


### Plan

- Process webhooks with a queue
  - [ ] Webhook endpoints publish to two topics: orders.webhooks, disputes.webhooks, keyed by user_id.
  - [ ] Add event_id to envelope for idempotency.
- Workers
  - [ ] Run 2 workers to consume and process these topics.
  - [ ] Ensure idempotent writes in DB (UPSERT / ON CONFLICT on event_id or natural key).
- Ingest Service 
   - [ ] Extract webhook handling into a separate service.
   - [ ] Add auth + schema validation (JSON Schema / Protobuf).
   - [ ] Invalid/unprocessable messages → DLQ with reason.
- Scale-out
   - [ ] Increase topic partitions; scale workers (1 worker per partition)..
   - [ ] Verify consistent routing by hash(user_id) across producers/consumers.


## Step 2: Outbox pattern → CDC → Analytics

## Step 3: Sharding experiments
