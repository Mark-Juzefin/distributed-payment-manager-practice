# Feature 001: Webhooks Ingestion with Kafka

**Status:** In Progress

## Overview

Replace synchronous webhook processing with Kafka-based async ingestion.

**Architecture:** [kafka-architecture.md](../kafka-architecture.md)

## Subtasks

**Subtask 1:** Basic Kafka integration — [plan](plan-subtask-1.md)
- [x] Webhook handlers publish to Kafka instead of sync processing
- [x] Workers consume topics and process events

**Subtask 2:** Sync/Kafka mode — [plan](plan-subtask-2.md)
- [x] Sync/Kafka mode switch via env variable
- [x] Integration test fixes for async mode

**Subtask 3:** Test isolation — [plan](plan-subtask-3.md)
- [x] Testcontainers instead of docker-compose

**Subtask 4:** Consumer resilience — [plan](plan-subtask-4.md)
- [x] Retry with exponential backoff + jitter
- [x] Panic recovery (defer + recover)
- [x] Dead Letter Queue for poison messages

**Subtask 5:** Sharding-ready architecture
- [ ] Partition key by user_id instead of order_id
- [ ] Add user_id to ChargebackWebhook (simplified — real providers won't have it)
- [ ] Separate ingest service binary

## Future: Realistic user_id lookup

Current approach is simplified — we add user_id directly to webhook payloads. In reality:
- External providers don't know our internal user_id
- Webhook might contain only email or order_id
- Need to lookup user_id before Kafka publish or on consume
- Opportunities: cache layer (Redis), lookup service, cross-shard queries

→ Consider for Step 3 (Sharding) or later.

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
