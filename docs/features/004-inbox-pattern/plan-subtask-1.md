# План: Subtask 1 — Shared Kernel Refactoring

## Мета

Розв'язати Ingest service від API domain types. Після рефакторингу Ingest має **нуль імпортів з `internal/api/`**. Обидва сервіси можуть імпортувати з `internal/shared/` та `pkg/`.

## Поточний стан

Ingest напряму імпортує з `internal/api/`:
- **Domain types**: `order.OrderUpdate`, `dispute.ChargebackWebhook` — для десеріалізації вебхуків
- **Domain errors**: `order.ErrInvalidStatus`, `order.ErrNotFound`, `dispute.ErrEventAlreadyStored`
- **Messaging**: `messaging.Envelope`, `messaging.Publisher`, `messaging.NewEnvelope()`
- **Kafka**: `kafka.NewPublisher()`

Вже існує шар декаплінгу — DTOs в `internal/shared/dto/` (використовуються в HTTP sync mode).

## Архітектурні рішення

| Питання | Рішення | Чому |
|---------|---------|------|
| Як позбутися domain types в Ingest? | Перейти на DTOs (`shared/dto`) скрізь | DTOs вже існують, wire format ідентичний (JSON tags збігаються) |
| Куди перенести messaging? | `internal/shared/messaging/` | Використовується обома сервісами, project-specific (не `pkg/`) |
| Куди перенести Kafka publisher? | `internal/shared/kafka/` | Infrastructure adapter, не бізнес-логіка API |
| Як обробляти помилки в handlers? | Використовувати `apiclient.Err*` | Вже існують в Ingest, в Kafka mode domain errors не повертаються |
| Чи зміниться Kafka wire format? | Ні | Domain types і DTOs серіалізуються ідентично |
| Чи потрібно змінювати API consumers? | Тільки import paths | Wire format той самий, десеріалізація в domain types працює |

## Імплементація

### Step 1: Перенести `internal/api/messaging/` → `internal/shared/messaging/`

Файли: `types.go`, `middleware.go`, `runner.go`

Оновити імпорти в:
- `internal/api/workers.go`
- `internal/api/consumers/order.go`, `dispute.go`
- `internal/api/external/kafka/publisher.go`, `consumer.go`

### Step 2: Перенести `internal/api/external/kafka/` → `internal/shared/kafka/`

Файли: `publisher.go`, `consumer.go`, `dlq_publisher.go`

Оновити імпорти в:
- `internal/api/workers.go`
- `internal/ingest/app.go`

### Step 3: Рефакторинг Ingest Processor на DTOs

**processor.go** — змінити сигнатуру інтерфейсу:
```go
type Processor interface {
    ProcessOrderUpdate(ctx context.Context, req dto.OrderUpdateRequest) error
    ProcessDisputeUpdate(ctx context.Context, req dto.DisputeUpdateRequest) error
}
```

**async.go** — DTOs + `shared/messaging`:
- Отримує `dto.OrderUpdateRequest`, загортає в `messaging.Envelope`
- Key для Kafka: `req.UserID`

**http.go** — спрощується до pass-through:
- Просто передає DTO в `client.SendOrderUpdate(ctx, req)`, без конвертації

**handlers/order.go** — біндить JSON до `dto.OrderUpdateRequest`, помилки через `apiclient.Err*`

**handlers/chargeback.go** — те саме для `dto.DisputeUpdateRequest`

### Step 4: Оновити тести

- `async_test.go`, `http_test.go` — переписати на DTOs
- E2E тести — БЕЗ змін (вебхуки відправляються як `map[string]interface{}`)

### Step 5: Видалити порожні директорії

- `internal/api/messaging/`
- `internal/api/external/kafka/`

## Файли для модифікації

| Файл | Зміни |
|------|-------|
| `internal/shared/messaging/*.go` | NEW (moved from `api/messaging/`) |
| `internal/shared/kafka/*.go` | NEW (moved from `api/external/kafka/`) |
| `internal/api/workers.go` | Update imports |
| `internal/api/consumers/order.go` | Update imports |
| `internal/api/consumers/dispute.go` | Update imports |
| `internal/ingest/app.go` | Update import: `shared/kafka` |
| `internal/ingest/webhook/processor.go` | Rewrite: DTO types |
| `internal/ingest/webhook/async.go` | Rewrite: DTOs + `shared/messaging` |
| `internal/ingest/webhook/http.go` | Simplify: pass-through |
| `internal/ingest/handlers/order.go` | Rewrite: DTO + `apiclient.Err*` |
| `internal/ingest/handlers/chargeback.go` | Rewrite: DTO + `apiclient.Err*` |
| `internal/ingest/webhook/async_test.go` | Update to DTOs |
| `internal/ingest/webhook/http_test.go` | Update to DTOs |

## Порядок імплементації

1. Move messaging (mechanical) → compile check
2. Move kafka (mechanical) → compile check
3. Refactor Ingest processor + handlers → compile check
4. Update Ingest tests → `make test`
5. Clean up empty dirs
6. Verify: `grep -r "internal/api/" internal/ingest/` = 0 results
7. `make integration-test`, `make e2e-test`, `make e2e-test-http`

## Wire Format Compatibility Proof

| Domain type field | JSON tag | DTO field | JSON tag | Compatible? |
|---|---|---|---|---|
| `OrderUpdate.OrderId` | `order_id` | `OrderUpdateRequest.OrderID` | `order_id` | ✅ |
| `OrderUpdate.UserId` | `user_id` | `OrderUpdateRequest.UserID` | `user_id` | ✅ |
| `OrderUpdate.Status` (type Status) | `status` | `OrderUpdateRequest.Status` (string) | `status` | ✅ |
| `ChargebackWebhook.Money.Amount` | `amount` | `DisputeUpdateRequest.Amount` | `amount` | ✅ |
| `ChargebackWebhook.Money.Currency` | `currency` | `DisputeUpdateRequest.Currency` | `currency` | ✅ |

Embedded `Money` struct серіалізується як flat fields — ідентично DTO.
