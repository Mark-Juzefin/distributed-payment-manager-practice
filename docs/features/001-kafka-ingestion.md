# Feature 001: Webhooks Ingestion with Kafka

**Status:** In Progress

## Overview

Replace synchronous webhook processing with Kafka-based async ingestion.

## Architecture

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

## Tasks

### Phase 1: Basic Kafka Integration
- [ ] Webhook endpoints publish to two topics: orders.webhooks, disputes.webhooks, keyed by user_id
- [ ] Add event_id to envelope for idempotency

### Phase 2: Workers
- [ ] Run 2 workers to consume and process these topics
- [ ] Ensure idempotent writes in DB (UPSERT / ON CONFLICT on event_id or natural key)

### Phase 3: Ingest Service
- [ ] Extract webhook handling into a separate service
- [ ] Add auth + schema validation (JSON Schema / Protobuf)
- [ ] Invalid/unprocessable messages → DLQ with reason

### Phase 4: Scale-out
- [ ] Increase topic partitions; scale workers (1 worker per partition)
- [ ] Verify consistent routing by hash(user_id) across producers/consumers

## Notes

### 2025-12-27: Pre-Kafka Groundwork (Preparation Phase)

**Completed:** Idempotency infrastructure fixes before Kafka integration

**Changes:**
- ✅ Added UNIQUE constraints for idempotency:
  - `order_events(order_id, provider_event_id)`
  - `dispute_events(dispute_id, provider_event_id, created_at)` *(includes partition key)*
- ✅ Fixed event creation error handling:
  - Events are now critical operations (webhook fails if event creation fails)
  - Repositories return `apperror.ErrEventAlreadyStored` on duplicate
  - Handlers properly check for duplicates via `errors.Is()`
- ✅ Fixed binding error handling in webhook handlers (added missing `return` statements)
- ✅ Added integration tests:
  - `TestCreateOrderEvent_IdempotencyConstraint` (order_eventsink)
  - `TestCreateDisputeEvent_IdempotencyConstraint` (dispute_eventsink)
- ✅ Created `.claude/rules/migrations.md` for migration testing standards

**Why this matters for Kafka:**
- Kafka consumers will retry failed messages
- UNIQUE constraints + webhook retry = safe idempotency
- Without these fixes, retries would create duplicate events

**Migration:** `20251227102937_add_idempotency_constraints.sql`

**Next:** Ready for Phase 1 - Basic Kafka Integration
