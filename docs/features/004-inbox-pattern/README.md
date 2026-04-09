# Inbox Pattern: Reliable Webhook Ingestion

**Status:** Paused

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

- [x] Subtask 1: Shared kernel refactoring — decouple Ingest from API domain types
- [x] Subtask 2: Inbox table + Ingest writes (separate Postgres for Ingest, raw JSONB payloads, return 200 OK)
- [x] Subtask 3: DB-queue worker (SKIP LOCKED) — Ingest poll worker reads inbox, forwards to API via HTTP, retry logic
- [ ] Subtask 3.1: Inbox e2e & integration tests — full flow webhook→inbox→worker→API→DB, worker with real DB, edge cases
- [ ] Subtask 4: CDC + Kafka variant — inbox + outbox in one TX, CDC publishes to Kafka, API consumes
- [ ] Subtask 5: Benchmarks & comparison — loadtest both approaches, latency/throughput metrics, trade-off analysis

## Notes

- CDC infrastructure from Feature 003 (Outbox) can be reused for Phase 2 inbox processing
- Ingest gets its own Postgres instance (separate from API's database)
- `SKIP LOCKED` is the standard Postgres pattern for job queues — avoids row-level contention between workers
- At-least-once delivery in both phases — idempotent processing required on API side
- Raw webhook payloads stored as JSONB — no domain type dependency at ingestion time

## Changelog

### Subtask 1: Shared kernel refactoring

**Shared DTO package:**
- Moved `OrderUpdateRequest`, `DisputeUpdateRequest` and response types to `services/*/dto/`
- Both API and Ingest import from shared package — no cross-service domain dependency

**Ingest decoupling:**
- Ingest handlers and webhook processors now use `dto.*` types instead of API domain types
- `apiclient.Client` interface uses shared DTOs for `SendOrderUpdate`/`SendDisputeUpdate`

### Subtask 2: Inbox table + Ingest writes

**Ingest-owned database:**
- Ingest gets its own PostgreSQL connection (`INGEST_PG_URL`, `INGEST_PG_POOL_MAX`)
- Separate migration system: `services/ingest/migrations/` with embedded FS (`ingest.MigrationFS`)
- Migrations applied on startup in `"inbox"` webhook mode

**Inbox table:**
- `inbox` table: `id UUID PK`, `idempotency_key`, `webhook_type`, `payload JSONB`, `status`, `received_at`, `processed_at`, `error_message`, `retry_count`
- Unique index on `idempotency_key` for idempotent writes
- Partial index on `(status, received_at) WHERE status = 'pending'` for efficient polling

**Inbox processor:**
- `webhook.InboxProcessor` — stores raw payload in inbox, returns nil on duplicate (idempotent)
- Idempotency key format: `{webhook_type}:{provider_event_id}`
- New webhook mode `"inbox"` in Ingest service alongside existing `"kafka"` and `"http"`

**New files:**
- `services/ingest/migration.go` — embed directive for migrations
- `services/ingest/migrations/20260214120000_create_inbox_table.sql`
- `services/ingest/repo/inbox/pg_inbox_repo.go` — `InboxRepo` interface + `PgInboxRepo`
- `services/ingest/webhook/inbox.go` — `InboxProcessor`
- Integration tests: `pg_inbox_repo_integration_test.go` (Store, idempotency constraint)

### Subtask 3: DB-queue worker (SKIP LOCKED)

**Inbox repo extensions:**
- `InboxMessage` struct — full row representation for fetched messages
- `FetchPending(ctx, limit)` — atomic claim via `UPDATE SET status='processing' WHERE id IN (SELECT ... FOR UPDATE SKIP LOCKED) RETURNING ...`
- `MarkProcessed(ctx, id)` — sets `status='processed'`, `processed_at=NOW()`
- `MarkFailed(ctx, id, errMsg, maxRetries)` — increments `retry_count`, resets to `'pending'` if under max retries, sets `'failed'` if exhausted

**Inbox worker (`services/ingest/worker/`):**
- `InboxWorker` — polls inbox on configurable interval, fetches batch, processes sequentially
- Dispatches by `webhook_type`: `"order_update"` → `client.SendOrderUpdate`, `"dispute_update"` → `client.SendDisputeUpdate`
- Error classification: `ErrConflict` → treat as success (idempotent); `ErrBadRequest`/`ErrNotFound`/`ErrInvalidStatus` → permanent failure (maxRetries=0); `ErrServiceUnavailable` → transient (retry via pending reset)
- Graceful shutdown via context cancellation

**Configuration:**
- `INBOX_POLL_INTERVAL` (default 100ms), `INBOX_BATCH_SIZE` (default 10), `INBOX_MAX_RETRIES` (default 5)

**Wiring:**
- Inbox mode in `app.go` now creates `apiclient.HTTPClient` + `InboxWorker`, starts worker in goroutine

**Migration:**
- `20260214130000_add_processing_status_index.sql` — partial index on `status='processing'` for stuck row recovery

**Tests:**
- 11 unit tests for worker: success/failure paths, conflict=idempotent, permanent vs transient errors, empty batch, context cancellation
- Generated mocks for `InboxRepo` and `apiclient.Client` via mockgen
