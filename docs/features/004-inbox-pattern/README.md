# Inbox Pattern: Reliable Webhook Ingestion

**Status:** Planned

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

## Scope

### Core: Inbox Pattern
- Inbox table in Ingest service's own database (or shared DB with separate schema)
- Raw webhook payload stored as JSONB (no domain type dependency)
- Background worker picks up unprocessed rows and forwards to API
- Status tracking: `pending` → `processing` → `done` / `failed`
- Retry logic for failed processing
- Cleanup/archival of processed rows

### Bonus: Shared Kernel Refactoring
- Extract `PaymentWebhook`, `ChargebackWebhook` to `internal/shared/` or define Ingest-own DTOs
- Move `kafka.Publisher` to `pkg/kafka/` or `internal/shared/kafka/`
- Ingest no longer imports anything from `internal/api/`

## Tasks

> Subtasks will be defined during planning phase.

- [ ] Subtask 1: TBD
- [ ] Subtask 2: TBD
- [ ] Subtask 3: TBD

## Notes

- CDC infrastructure from Feature 003 (Outbox) can potentially be reused for Inbox processing
- Decision needed: separate DB for Ingest vs shared DB with separate schema
- Decision needed: polling worker vs CDC for inbox processing
- Consider combining with DLQ improvements — failed inbox items vs current DLQ topic approach
