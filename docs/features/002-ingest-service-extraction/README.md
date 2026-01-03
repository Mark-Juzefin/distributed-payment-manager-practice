# Feature 002: Ingest Service Extraction

**Status:** In Progress

## Overview

Виділення Ingest service як окремого мікросервісу. Перший крок до мікросервісної архітектури (Step 5 roadmap).

**Архітектура:**

```
Kafka mode (production):
  Webhook → Ingest (HTTP) → Kafka → API consumer → domain logic

Sync mode (dev, без Kafka):
  Webhook → API (HTTP) → domain logic
```

Consumer завжди в API service (retry через Kafka re-delivery). Sync mode не потребує Ingest service.

## Subtasks

**Subtask 0:** Preparation — [plan-subtask-0.md](plan-subtask-0.md)
- [x] Domain errors refactoring
- [ ] Integration tests improvements (t.Parallel)
- [ ] Minor cleanups (ChargebackHandler merge, typed Gateway errors)

**Subtask 1:** Ingest Service with Kafka mode — [plan-subtask-1.md](plan-subtask-1.md)
- [x] Створити `cmd/ingest/` binary
- [x] HTTP → Kafka gateway (легкий edge service)
- [x] API consumer читає з Kafka → domain logic
- [x] Два окремих процеси: Ingest + API
- [x] Deployment configs (Makefile, Docker)

**Subtask 1.5:** Monorepo Architecture Refactoring — [plan-subtask-1.5.md](plan-subtask-1.5.md)
- [x] Реорганізувати `internal/` на service-based структуру
- [x] Перемістити shared код in `internal/shared/`
- [x] Розділити handlers (API vs Ingest, без nullable deps)
- [x] Оновити routers та bootstrap

**Subtask 2:** gRPC for sync mode
- [ ] gRPC proto definitions
- [ ] gRPC server в API service
- [ ] Ingest викликає gRPC замість Kafka (sync mode)
- [ ] WEBHOOK_MODE switch: `kafka` vs `sync`

**Subtask 3:** Observability basics (optional)
- [ ] Health checks для обох сервісів
- [ ] Structured logging з correlation IDs
- [ ] Basic metrics (Prometheus-ready)

---

## Architecture Decision Records

### ADR-1: Consumer placement

**Decision:** Kafka consumer в API service, не в Ingest.

**Rationale:**
- Retry природній через Kafka re-delivery (не commit offset → re-process)
- Transaction boundaries чіткі (1 DB transaction per message)
- Domain визначає transient vs permanent errors
- Менше distributed complexity

### ADR-2: gRPC for sync mode only

**Decision:** gRPC використовується тільки для sync mode, не як прослойка після Kafka.

**Rationale:**
- Якщо consumer в Ingest викликає gRPC після Kafka:
  - Складні retries (gRPC timeout ≠ domain error)
  - Потрібен idempotency key на кожен call
  - Distributed transaction problem
- Sync mode потребує синхронний call → gRPC ідеально

---

## Notes

### 2024-12-30: Initial Analysis

Провели аналіз архітектури. Головні проблеми:
1. Domain залежить від controller через `apperror`
2. Monolith binary не дозволяє окремо скейлити компоненти

### 2026-01-01: Post-Kafka Review

Після завершення Kafka integration (Feature 001) частина проблем виправлена:
- Testcontainers замість docker-compose
- TestMain() для міграцій
- testinfra package для shared setup

### 2026-01-01: Subtask 0 - Domain errors complete

Domain errors refactoring завершено:
- Створено `internal/domain/order/errors.go` (7 errors)
- Створено `internal/domain/dispute/errors.go` (2 errors)
- Оновлено 16 файлів: domain, repo, controller layers + integration tests
- Видалено `internal/controller/apperror/`
- Domain layer тепер повністю незалежний від controller

### 2026-01-02: Feature restructure

Фічу перейменовано з "Architecture Review & Refactoring" на "Ingest Service Extraction".
Фокус змістився з розрізнених рефакторингів на чітку ціль - створення окремого сервісу.

### 2026-01-03: Subtask 1 complete - Ingest Service implemented

Успішно створено Ingest Service:
- Створено `cmd/ingest/` та `cmd/api/` entry points
- Імплементовано `internal/ingest/ingest.go` - легкий HTTP → Kafka gateway
- Рефакторинг `internal/app/app.go` для підтримки kafka mode без webhook endpoints
- Створено `WebhookRouter` для Ingest та `APIRouter` для API service
- Handlers з nullable dependencies (service/processor can be nil)
- Deployment: Makefile, Procfile, Dockerfiles, env files
- Оновлено документацію: README.md, CLAUDE.md

**Архітектурні рішення:**
- Sync mode: тільки API service (WEBHOOK_MODE=sync)
- Kafka mode: обидва сервіси через goreman
- Kafka consumers залишились в API service
- Ingest не має доступу до domain logic чи БД

### 2026-01-03: Subtask 1.5 complete - Monorepo architecture refactored

Успішно виконано рефакторинг архітектури з layer-based на service-based:

**Нова структура:**
- `internal/api/` - API service (handlers, consumers, router, bootstrap)
- `internal/ingest/` - Ingest service (webhook handlers, router, bootstrap)
- `internal/shared/` - Shared code (domain, repo, external, webhook, messaging, testinfra)

**Ключові зміни:**
- Чисті handlers без nullable dependencies
- API handlers мають тільки service (без processor)
- Ingest handlers мають тільки processor (без service)
- API router без `includeWebhooks` boolean flag
- Kafka consumers в `internal/api/consumers/`
- Migrations в `internal/api/migrations/`
- Видалено `internal/controller/` та `internal/app/`

**Sync mode support:**
- API service динамічно додає webhook endpoints в sync mode
- Використовує ingest handlers з SyncProcessor

**Testing:**
- Всі unit tests пройшли ✅
- Integration tests оновлено під нову структуру
- CLAUDE.md оновлено з новою архітектурою
