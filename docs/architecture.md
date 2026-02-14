# Kafka Webhooks Architecture

Target architecture for async webhook processing with Kafka.

```mermaid
flowchart LR
  %% External → Ingest
  EXT["External Provider<br/>webhooks: orders, disputes"]
    -->|HTTP| ING["Ingest API<br/>(auth, validation)"]

  %% Kafka: key = user_id
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

## Components

| Component | Description |
|-----------|-------------|
| **Ingest API** | HTTP endpoint for webhooks, validates and publishes to Kafka |
| **Kafka Topics** | `orders.webhooks`, `disputes.webhooks` - partitioned by `order_id` |
| **Consumer Groups** | One per topic, workers consume in parallel |
| **DLQ** | Dead Letter Queue for failed messages (future) |
| **Sharded Postgres** | Target architecture for scale-out (future) |
