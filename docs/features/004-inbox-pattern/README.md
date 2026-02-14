# Inbox Pattern: Reliable Webhook Ingestion

**Status:** In Progress

## Overview

Implement the **Inbox Pattern** for reliable webhook ingestion. Instead of forwarding webhooks directly to Kafka or API, the Ingest service saves the raw webhook payload to a durable inbox table first, returns `200 OK` immediately, then processes asynchronously via a background worker (polling or CDC).

This is the mirror of the Outbox Pattern (Feature 003): Outbox handles **outgoing** events reliably, Inbox handles **incoming** events reliably.

**Motivation:**
- Current system loses webhooks if Kafka is down at the moment of ingestion
- Ingest service has tight coupling to API domain types and Kafka infrastructure
- No durable record of raw incoming webhooks for debugging/replay
- No backpressure mechanism — spike in webhooks directly pressures downstream systems

## Architecture

```
Current flow:
  Provider → Ingest → Kafka → API consumer → DB
  (if Kafka down → webhook lost, relies on provider retry)

With Inbox Pattern:
  Provider → Ingest → INSERT into inbox table → return 200 OK
                              ↓
                   CDC / polling worker
                              ↓
                   API business logic → DB
```

## Key Concepts to Practice

- **Inbox Pattern** — transactional receive, store-and-forward, at-least-once delivery
- **Store-and-Forward** — decouple ingestion speed from processing speed
- **Backpressure** — inbox acts as a buffer; processing happens at system's own pace
- **Idempotent processing** — inbox rows may be processed more than once (at-least-once)
- **Shared Kernel refactoring** — decouple Ingest from API domain types as part of the pattern
- **Replay & debugging** — full history of raw incoming webhooks

## Implementation Phases

### Phase 1: DB-queue (Postgres + SKIP LOCKED)
Simple polling approach — Ingest writes to inbox table, API poll worker reads with `SELECT ... FOR UPDATE SKIP LOCKED`.

### Phase 2: CDC + Kafka (reuse Step 3 infrastructure)
Production-style — Ingest writes inbox + outbox in one transaction, CDC publishes outbox to Kafka, API consumes from Kafka.

### Phase 3: Benchmarks & comparison
Load test both approaches, measure latency/throughput, document trade-offs.

## Architectural Decisions

| Decision | Choice | Reasoning |
|----------|--------|-----------|
| Ingest database | Separate Postgres instance | Ingest owns its data, no shared-DB coupling |
| Shared kernel refactoring | Separate subtask | Clean boundary before inbox implementation |
| Phase 1 approach | DB-queue (SKIP LOCKED) | Simpler, fewer moving parts, good baseline |
| Phase 2 approach | CDC + Kafka | Reuse Step 3 CDC infra, compare with Phase 1 |

## Tasks

- [x] Subtask 1: Shared kernel refactoring — decouple Ingest from API domain types — [plan](plan-subtask-1.md)
- [ ] Subtask 2: Inbox table + Ingest writes (separate Postgres for Ingest, raw JSONB payloads, return 200 OK)
- [ ] Subtask 3: DB-queue worker (SKIP LOCKED) — API poll worker reads inbox, processes, updates status, retry logic
- [ ] Subtask 4: CDC + Kafka variant — inbox + outbox in one TX, CDC publishes to Kafka, API consumes
- [ ] Subtask 5: Benchmarks & comparison — loadtest both approaches, latency/throughput metrics, trade-off analysis

## Notes

- CDC infrastructure from Feature 003 (Outbox) can be reused for Phase 2 inbox processing
- Ingest gets its own Postgres instance (separate from API's database)
- `SKIP LOCKED` is the standard Postgres pattern for job queues — avoids row-level contention between workers
- At-least-once delivery in both phases — idempotent processing required on API side
- Raw webhook payloads stored as JSONB — no domain type dependency at ingestion time
