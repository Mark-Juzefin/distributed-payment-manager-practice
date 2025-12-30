# Feature 002: Architecture Review

**Status:** In Progress

## Overview

Аналіз поточної архітектури на предмет потенційних проблем масштабування по мірі росту кодової бази.

## Tasks

- [x] Аналіз організації шарів (`internal/` structure)
- [x] Аналіз проблем з інтеграційними тестами
- [ ] Документація findings та рекомендацій
- [ ] План рефакторингу (якщо потрібен)

---

## Findings

### Layer Organization Issues

#### 1. Domain залежить від Controller (КРИТИЧНО)

**Проблема:** Domain layer імпортує `apperror` з controller:

```go
// internal/domain/order/order_entity.go
import "TestTaskJustPay/internal/controller/apperror"

// internal/domain/order/service.go
if len(orders) == 0 {
    return Order{}, apperror.ErrOrderNotFound
}
```

**Імпакт:**
- Неможливо додати gRPC/GraphQL без змін у domain
- Тестування domain залежить від controller структур
- Порушує Dependency Inversion Principle

**Рішення:** Створити `internal/domain/errors/` або errors в кожному bounded context.

---

#### 2. Event Sink реалізації розкидані (ВИСОКА)

**Проблема:**
```
internal/repo/order_eventsink/     ← PostgreSQL
internal/repo/dispute_eventsink/   ← PostgreSQL
internal/external/opensearch/      ← OpenSearch (теж EventSink!)
```

Обидва реалізують `dispute.EventSink`, але розташовані в різних шарах.

**Рішення:** Консолідувати всі persistence реалізації в `repo/`.

---

#### 3. Messaging без транзакційних гарантій (ВИСОКА)

**Проблема:**
```go
// REST handler
err := h.service.ProcessPaymentWebhook(ctx, webhook)  // ✓ Success
err := h.publisher.Publish(ctx, envelope)              // ✗ Fail - що робити?
```

Якщо publish fail після успішної обробки - немає rollback або retry.

**Рішення:** Event publishing має бути частиною domain service або використовувати Outbox pattern.

---

#### 4. Gateway errors без типізації (СЕРЕДНЯ)

**Проблема:** Silvergate повертає generic `error`, service не може розрізнити:
- Transient (retry) - network timeout
- Permanent (abort) - account suspended
- Validation (reject) - invalid card

**Рішення:** Typed errors в `gateway/errors.go`.

---

#### 5. Chargeback як окремий handler (НИЗЬКА)

**Проблема:** `ChargebackHandler` використовує той самий `DisputeService`, що й `DisputeHandler`. Це не окремий bounded context.

**Рішення:** Об'єднати в `DisputeHandler.Chargeback()`.

---

### Integration Tests Issues

#### 1. TRUNCATE замість SandboxTransaction (КРИТИЧНО)

**Проблема:**
```go
// integration-test/integration_test.go
pool.Pool.Exec(ctx, "TRUNCATE TABLE orders, disputes, ...")
```

При падінні тесту наступний бачить неповні дані.

**Рішення:** Використовувати `SandboxTransaction` як у repo тестах.

---

#### 2. Немає t.Parallel() в E2E (ВИСОКА)

**Проблема:** 9 тестів в `integration-test/` запускаються послідовно.

**Рішення:** Додати `t.Parallel()` + ізоляцію через transactions.

---

#### 3. Міграції запускаються для кожного тесту (ВИСОКА)

**Проблема:**
```go
func setupTestServer(t *testing.T) {
    app.ApplyMigrations(cfg.PgURL, ...)  // Кожен раз!
}
```

**Рішення:** Винести в `TestMain()`.

---

#### 4. Дублювання setup коду (СЕРЕДНЯ)

**Проблема:** `applyBaseFixture()` визначена 3 рази в різних пакетах. Container setup дублюється.

**Рішення:** Спільний `internal/testutil/` пакет.

---

#### 5. order_eventsink не в Makefile (НИЗЬКА)

**Проблема:**
```makefile
INTEGRATION_DIRS = \
    ./internal/repo/dispute_eventsink \
    ./integration-test/...
# order_eventsink пропущено!
```

---

## Priority Matrix

| Issue | Severity | Effort | Impact on Growth |
|-------|----------|--------|------------------|
| Domain → Controller dependency | CRITICAL | Large | Blocks new UI layers |
| TRUNCATE in E2E tests | CRITICAL | Medium | Test isolation failures |
| Event sink placement | HIGH | Medium | Confusion in persistence |
| No t.Parallel() in E2E | HIGH | Small | Slow CI |
| Messaging guarantees | HIGH | Large | Data consistency |
| Migrations per test | HIGH | Medium | Slow tests |
| Gateway error types | MEDIUM | Small | No retry policy |
| Setup code duplication | MEDIUM | Medium | Maintenance burden |
| Chargeback handler | LOW | Small | Code organization |

---

## Notes

### 2024-12-30: Initial Analysis

Провели глибокий аналіз архітектури. Головна проблема - domain layer має зворотну залежність від controller через `apperror`. Це блокує будь-яке розширення UI layer (gRPC, GraphQL) без змін у core domain.

Інтеграційні тести мають архітектурну проблему: E2E тести не використовують ізоляцію через transactions, що робить їх flaky.

**Наступний крок:** Обговорити з користувачем пріоритети та план рефакторингу.
