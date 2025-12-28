# Feature 001: Webhooks Ingestion with Kafka

**Status:** In Progress

## Overview

Replace synchronous webhook processing with Kafka-based async ingestion.

**Architecture:** [kafka-architecture.md](../kafka-architecture.md)

**Subtask 1 план:** [plan-subtask-1.md](plan-subtask-1.md)

## Tasks

- [ ] Інфраструктура (Redpanda + topics)
- [ ] Config (brokers, topics, consumer groups)
- [ ] Messaging абстракції (`internal/messaging/`)
- [ ] Kafka implementation (`internal/external/kafka/`)
- [ ] Message Controllers (`internal/controller/message/`)
- [ ] Handler модифікації (publish замість sync)
- [ ] App wiring + graceful shutdown

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
