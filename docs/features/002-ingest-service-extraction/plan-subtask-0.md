# План: Domain errors refactoring

## Мета

Перенести помилки з `internal/controller/apperror/` до domain layer, щоб domain не залежав від controller. Використовуємо підхід "errors per bounded context" - кожен домен має свої помилки.

## Поточний стан

```
internal/controller/apperror/apperror.go  ← Всі помилки тут
├── ErrOrderNotFound
├── ErrOrderAlreadyExists
├── ErrUnappropriatedStatus
├── ErrOrderOnHold
├── ErrOrderInFinalStatus
├── ErrInvalidOrdersQuery
├── ErrDisputeAlreadyExists
└── ErrEventAlreadyStored
```

**Порушення:** Domain імпортує controller:
```go
// internal/domain/order/service.go
import "TestTaskJustPay/internal/controller/apperror"  // ← BAD
```

## Архітектурні рішення

| Питання | Рішення | Чому |
|---------|---------|------|
| Один пакет чи per-context? | Per bounded context | Кожен домен ізольований, краще для мікросервісів |
| Де зберігати shared errors? | Дублювати або винести в `pkg/` | `ErrEventAlreadyStored` використовується в обох контекстах |
| Що робити з `apperror`? | Видалити після міграції | Залишати deprecated код - технічний борг |

## Цільова структура

```
internal/domain/
├── order/
│   ├── errors.go          ← NEW: Order-specific errors
│   ├── service.go
│   ├── order_entity.go
│   └── ...
├── dispute/
│   ├── errors.go          ← NEW: Dispute-specific errors
│   └── ...
└── ...

internal/controller/apperror/  ← DELETE after migration
```

## Розподіл помилок

### `internal/domain/order/errors.go`
```go
package order

import "errors"

var (
    ErrNotFound            = errors.New("order not found")
    ErrAlreadyExists       = errors.New("order already exists")
    ErrInvalidStatus       = errors.New("invalid status transition")
    ErrOnHold              = errors.New("order is on hold")
    ErrInFinalStatus       = errors.New("order is in final status")
    ErrInvalidQuery        = errors.New("invalid orders query")
    ErrEventAlreadyStored  = errors.New("event already stored")
)
```

### `internal/domain/dispute/errors.go`
```go
package dispute

import "errors"

var (
    ErrAlreadyExists       = errors.New("dispute already exists")
    ErrEventAlreadyStored  = errors.New("event already stored")
)
```

**Примітка:** `ErrEventAlreadyStored` дублюється в обох контекстах - це нормально для bounded contexts. Кожен контекст незалежний.

## Файли для модифікації

| Файл | Зміни |
|------|-------|
| `internal/domain/order/errors.go` | CREATE - нові помилки |
| `internal/domain/dispute/errors.go` | CREATE - нові помилки |
| `internal/domain/order/service.go` | Замінити `apperror.X` на `order.X` |
| `internal/domain/order/order_entity.go` | Замінити `apperror.X` на `order.X` |
| `internal/domain/order/event_sink.go` | Оновити коментар |
| `internal/domain/dispute/event_sink.go` | Оновити коментар |
| `internal/repo/order/pg_order_repo.go` | Замінити на `order.ErrAlreadyExists` |
| `internal/repo/order_eventsink/pg_order_event_sink.go` | Замінити на `order.ErrEventAlreadyStored` |
| `internal/repo/dispute_eventsink/pg_dispute_event_sink.go` | Замінити на `dispute.ErrEventAlreadyStored` |
| `internal/controller/rest/handlers/order.go` | Замінити на `order.X` |
| `internal/controller/rest/handlers/chargeback.go` | Перевірити використання |
| `internal/controller/message/order.go` | Замінити на `order.X` |
| `internal/controller/message/dispute.go` | Замінити на `dispute.X` |
| `internal/repo/order_eventsink/pg_order_event_sink_integration_test.go` | Оновити imports |
| `internal/repo/dispute_eventsink/pg_event_sink_integration_test.go` | Оновити imports |
| `internal/controller/apperror/apperror.go` | DELETE |

## Порядок імплементації

1. **Створити `internal/domain/order/errors.go`**
   - Визначити всі order-related помилки
   - Використати коротші імена (без префіксу `Order`)

2. **Створити `internal/domain/dispute/errors.go`**
   - Визначити dispute-related помилки

3. **Оновити domain layer**
   - `internal/domain/order/service.go` - видалити import apperror, використати локальні помилки
   - `internal/domain/order/order_entity.go` - аналогічно
   - Оновити коментарі в `event_sink.go` файлах

4. **Оновити repo layer**
   - `internal/repo/order/pg_order_repo.go`
   - `internal/repo/order_eventsink/pg_order_event_sink.go`
   - `internal/repo/dispute_eventsink/pg_dispute_event_sink.go`

5. **Оновити controller layer**
   - `internal/controller/rest/handlers/order.go`
   - `internal/controller/rest/handlers/chargeback.go`
   - `internal/controller/message/order.go`
   - `internal/controller/message/dispute.go`

6. **Оновити тести**
   - Integration tests для eventsinks

7. **Видалити `internal/controller/apperror/`**
   - Переконатись що ніхто не імпортує
   - Видалити директорію

8. **Перевірка**
   - `make lint`
   - `make test`
   - `make integration-test`

## Naming Convention

Старі імена → Нові імена:

| apperror | order/dispute package |
|----------|----------------------|
| `ErrOrderNotFound` | `order.ErrNotFound` |
| `ErrOrderAlreadyExists` | `order.ErrAlreadyExists` |
| `ErrUnappropriatedStatus` | `order.ErrInvalidStatus` |
| `ErrOrderOnHold` | `order.ErrOnHold` |
| `ErrOrderInFinalStatus` | `order.ErrInFinalStatus` |
| `ErrInvalidOrdersQuery` | `order.ErrInvalidQuery` |
| `ErrDisputeAlreadyExists` | `dispute.ErrAlreadyExists` |
| `ErrEventAlreadyStored` | `order.ErrEventAlreadyStored` / `dispute.ErrEventAlreadyStored` |

Коротші імена без префіксів - бо пакет вже дає контекст (`order.ErrNotFound` замість `order.ErrOrderNotFound`).
