# Plan: Subtask 2 — Unified events table + atomic writes

## Context

After Subtask 1 (Transactor refactoring), services own their transactions and can include multiple participants. Now we add the unified `events` table and write to it atomically alongside business data. Old `order_events` / `dispute_events` tables stay untouched (Strangler Fig).

## Approach

```go
err := s.transactor.InTransaction(ctx, func(tx postgres.Executor) error {
    txRepo := s.txOrderRepo(tx)
    txEventStore := s.txEventStore(tx)        // NEW — same transaction

    txRepo.CreateOrder(ctx, update)
    txEventStore.CreateEvent(ctx, newEvent)   // atomic with business data
    return nil
})
s.orderEvents.CreateOrderEvent(ctx, update)   // old path, unchanged
```

## Implementation Steps

### Step 1: Migration — create `events` table

**New file:** `internal/api/migrations/YYYYMMDDHHMMSS_create_unified_events_table.sql`

```sql
CREATE TABLE public.events (
    id                 UUID         NOT NULL DEFAULT gen_random_uuid(),
    aggregate_type     VARCHAR(32)  NOT NULL,   -- 'order' | 'dispute'
    aggregate_id       VARCHAR(255) NOT NULL,   -- order_id or dispute_id
    event_type         VARCHAR(64)  NOT NULL,   -- 'webhook_received', 'hold_set', etc.
    idempotency_key    VARCHAR(255) NOT NULL,   -- provider_event_id or generated UUID
    payload            JSONB        NOT NULL,
    created_at         TIMESTAMPTZ  NOT NULL DEFAULT NOW(),
    CONSTRAINT events_pk PRIMARY KEY (id)
);
```

- **No partitioning** — plain table for now; partitioning will be added in a separate subtask
- Idempotency: `UNIQUE (aggregate_type, aggregate_id, idempotency_key)`
- Indexes: `(aggregate_type, aggregate_id, created_at)`, `(event_type, created_at)`
- No FK (generic entity_id, decoupled from business tables)
- `TIMESTAMPTZ` (best practice for time-series)
- Down: `DROP TABLE events CASCADE`

### Step 2: Domain types — `internal/api/domain/events/`

**New files:**

`event.go` — types and interface:
```go
type AggregateType string // "order" | "dispute"
type NewEvent struct { AggregateType, AggregateID, EventType, IdempotencyKey, Payload, CreatedAt }
type Event struct { ID string; NewEvent }
type Store interface {
    CreateEvent(ctx context.Context, event NewEvent) (*Event, error)
}
```

`errors.go`:
```go
var ErrEventAlreadyStored = errors.New("event already stored")
```

### Step 3: Event store implementation — `internal/api/repo/events/`

**New file:** `pg_event_store.go`
- `PgEventStore` struct with `postgres.Executor` + `squirrel.StatementBuilderType`
- `NewPgEventStore(db postgres.Executor, builder squirrel.StatementBuilderType)` constructor
- `TxStoreFactory(builder squirrel.StatementBuilderType)` — partial application factory (same pattern as `order_repo.TxRepoFactory`)
- `CreateEvent` — INSERT into `events`, handle unique violation → `ErrEventAlreadyStored`
- ID generation: `uuid.New().String()` (consistent with existing repos)

### Step 4: Wire event store into services

**Modify:** `internal/api/domain/order/service.go`
- Add `txEventStore func(tx postgres.Executor) events.Store` field
- Inside each `transactor.InTransaction` callback: create `txEventStore` and write unified event

**Modify:** `internal/api/domain/dispute/service.go` — same pattern.

**Modify:** `internal/api/app.go`
```go
// Services
orderService := order.NewOrderService(pool, order_repo.TxRepoFactory(pool.Builder), events_repo.TxStoreFactory(pool.Builder), orderRepo, silvergateClient, orderEvents)
disputeService := dispute.NewDisputeService(pool, dispute_repo.TxRepoFactory(pool.Builder), events_repo.TxStoreFactory(pool.Builder), disputeRepo, silvergateClient, disputeEvents)
```

### Step 5: Update test infrastructure

**Modify:** `testinfra/postgres.go` — add `events` to TRUNCATE statement
**Modify:** `Makefile` — add `events` to `clean-db` TRUNCATE

### Step 6: Integration tests for unified event store

**New files:** `internal/api/repo/events/pg_event_store_integration_test.go`, `integration_test.go`, `testdata/base.sql`

Tests:
- Event creation succeeds
- Idempotency constraint rejects duplicates (same aggregate_type, aggregate_id, idempotency_key)
- Different aggregates with same idempotency key succeed
- Returns `ErrEventAlreadyStored` on duplicate

## Files Summary

| File | Action | What changes |
|------|--------|-------------|
| `internal/api/migrations/YYYYMMDDHHMMSS_create_unified_events_table.sql` | NEW | Unified events table (no partitioning) |
| `internal/api/domain/events/event.go` | NEW | Domain types, Store interface |
| `internal/api/domain/events/errors.go` | NEW | ErrEventAlreadyStored |
| `internal/api/repo/events/pg_event_store.go` | NEW | PG implementation |
| `internal/api/repo/events/pg_event_store_integration_test.go` | NEW | Integration tests |
| `internal/api/repo/events/integration_test.go` | NEW | TestMain, container setup |
| `internal/api/repo/events/testdata/base.sql` | NEW | Test fixtures |
| `internal/api/domain/order/service.go` | MODIFY | Add eventStoreFactory, write events in tx |
| `internal/api/domain/dispute/service.go` | MODIFY | Same |
| `internal/api/app.go` | MODIFY | Wire eventStoreFactory |
| `testinfra/postgres.go` | MODIFY | Add `events` to TRUNCATE |
| `Makefile` | MODIFY | Add `events` to clean-db |

## What stays unchanged

- Old `order_events` / `dispute_events` tables and their `orderEvents`/`disputeEvents` writes
- Read paths (`GET /orders/events`, `GET /disputes/events`) — still use old tables
- API handlers, Kafka consumers

## Verification

1. `make test` — unit tests pass
2. `make integration-test` — new event store integration tests pass + existing eventsink tests
3. `make e2e-test` — E2E tests pass (old behavior preserved)
4. Manual: `make run-dev`, send webhook, verify row appears in both `order_events` AND `events` tables
