# Feature 001: Webhooks Ingestion with Kafka

**Status:** Done

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

**Subtask 5:** Sharding-ready architecture — [plan](plan-subtask-5.md)
- [x] Partition key by user_id instead of order_id
- [x] Add user_id to ChargebackWebhook (simplified — real providers won't have it)

> **Note:** "Separate ingest service binary" moved to [002-architecture-review](../002-architecture-review/README.md)

## Future: Realistic user_id lookup

Current approach is simplified — we add user_id directly to webhook payloads. In reality:
- External providers don't know our internal user_id
- Webhook might contain only email or order_id
- Need to lookup user_id before Kafka publish or on consume
- Opportunities: cache layer (Redis), lookup service, cross-shard queries

→ Consider for Step 3 (Sharding) or later.

## Notes

### 2026-01-01: Partition Key by user_id (Subtask 5, items 1-2)

**Completed:** Kafka partition key змінено з `order_id` на `user_id` для підготовки до шардингу.

**Changes:**
- ✅ Додано `UserID string` поле до `ChargebackWebhook` (`internal/domain/dispute/chargeback_entity.go`)
- ✅ Змінено partition key в `AsyncProcessor` (`internal/webhook/async.go`):
  - `ProcessOrderWebhook`: `webhook.UserId` замість `webhook.OrderId`
  - `ProcessDisputeWebhook`: `webhook.UserID` замість `webhook.OrderID`
- ✅ Оновлено логування в `DisputeMessageController` — додано `user_id` до всіх логів
- ✅ Оновлено інтеграційні тести — додано `user_id` до chargeback payloads

**Why this matters for sharding:**
- При шардингу БД по `user_id` всі дані користувача на одному шарді
- Kafka partition key = `user_id` гарантує ordering per user
- Consumer обробляє події користувача в правильному порядку
- Cross-shard queries мінімізовані

**Next:** Separate ingest service binary (item 3)

---

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
