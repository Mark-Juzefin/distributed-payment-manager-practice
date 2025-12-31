# Feature 001: Webhooks Ingestion with Kafka

**Status:** In Progress

## Overview

Replace synchronous webhook processing with Kafka-based async ingestion.

**Architecture:** [kafka-architecture.md](../kafka-architecture.md)

**Subtask 1 план:** [plan-subtask-1.md](plan-subtask-1.md)

## Tasks

- [x] Інфраструктура (Kafka + Zookeeper)
- [x] Config (brokers, topics, consumer groups)
- [x] Messaging абстракції (`internal/messaging/`)
- [x] Kafka implementation (`internal/external/kafka/`)
- [x] Message Controllers (`internal/controller/message/`)
- [x] Handler модифікації (publish замість sync)
- [x] App wiring + graceful shutdown

**Subtask 2 план:** [plan-subtask-2.md](plan-subtask-2.md)

- [x] Конфігурація режиму (sync/kafka) через env змінну
- [x] Оновити інтеграційний тест, щоб він не розраховував на синхронність вебхуків
- [x] Виправити проблему з consumer group offset в тестах (duplicate key через старі повідомлення)
- [x] Виправити retry в тестах для пустих результатів (GET повертає 200 з [])
- [x] Додати retry в consumer при order not found (race condition)
- [ ] Дослідити Transactional Outbox pattern для reliable messaging
- [ ] додати окремі інтеграційні тести до модулів кафки

**Subtask 3 план:** [plan-subtask-3.md](plan-subtask-3.md)

- [x] Testcontainers для ізоляції тестів (замість docker-compose для тестів)

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
