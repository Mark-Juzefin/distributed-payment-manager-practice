# Feature 002: Architecture Review & Refactoring

**Status:** In Progress

## Overview

Виправлення архітектурних проблем, виявлених під час розробки Kafka ingestion. Фокус на чистій архітектурі та підготовці до мікросервісів.

## Subtasks

**Subtask 1:** Domain errors refactoring — [plan-subtask-1.md](plan-subtask-1.md) ✅
- [x] Перенести `apperror` з controller до domain layer
- [x] Domain не має залежати від controller

**Subtask 2:** Separate ingest service binary
- [ ] Винести Kafka consumers в окремий `cmd/ingest/`
- [ ] Спільний код в `internal/` (domain, repo)
- [ ] Окремі бінарники: API server + Ingest workers

**Subtask 3:** Integration tests improvements
- [ ] Додати `t.Parallel()` до E2E тестів
- [ ] Ізоляція через unique IDs замість shared state

**Subtask 4:** Minor cleanups (optional)
- [ ] Об'єднати ChargebackHandler з DisputeHandler
- [ ] Typed errors для Gateway (retry vs permanent)
- [ ] Консолідувати EventSink реалізації в `repo/`

---

## Findings (Reference)

### Fixed Issues

| Issue | Resolution |
|-------|------------|
| Messaging guarantees | Kafka async mode + retry/DLQ. Outbox planned for Step 2 |
| TRUNCATE в E2E tests | Testcontainers з proper cleanup |
| Migrations per test | TestMain() в `internal/testinfra` |
| Setup code duplication | `internal/testinfra` package |

### Open Issues

#### 1. Domain → Controller dependency (CRITICAL)

```go
// internal/domain/order/service.go
import "TestTaskJustPay/internal/controller/apperror"  // ← Порушення!
```

**Чому критично:**
- Domain має бути framework-agnostic
- Неможливо додати gRPC/GraphQL без змін у domain
- Порушує Dependency Inversion Principle

**Рішення:** `internal/domain/errors/` або errors per bounded context.

#### 2. Monolith binary

```
cmd/app/main.go  ← Один бінарник робить все
├── HTTP server
├── Kafka consumers
└── Background workers
```

**Чому проблема:**
- Не можна скейлити consumers окремо від API
- Один crash валить все
- Складніший deployment

**Рішення:** Окремі бінарники `cmd/api/` та `cmd/ingest/`.

#### 3. No t.Parallel() in E2E tests

Тести в `integration-test/` запускаються послідовно. З testcontainers це безпечно паралелити.

#### 4. Gateway errors без типізації

Silvergate повертає generic `error`. Немає можливості розрізнити:
- Transient (retry) — network timeout
- Permanent (abort) — account suspended

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

Залишається: domain errors + separate binaries + test parallelization.

### 2026-01-01: Subtask 1 Complete

Domain errors refactoring завершено:
- Створено `internal/domain/order/errors.go` (7 errors)
- Створено `internal/domain/dispute/errors.go` (2 errors)
- Оновлено 16 файлів: domain, repo, controller layers + integration tests
- Видалено `internal/controller/apperror/`
- Domain layer тепер повністю незалежний від controller
- Використано підхід "errors per bounded context" для кращої ізоляції
- Всі тести пройшли успішно (unit + integration)
