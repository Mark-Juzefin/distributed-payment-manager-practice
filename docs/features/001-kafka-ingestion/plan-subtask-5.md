# План: Sharding-ready architecture (пункти 1-2)

## Мета

Підготувати Kafka partitioning до майбутнього шардингу — використовувати `user_id` як partition key замість `order_id`. Це забезпечить, що всі події одного користувача потраплятимуть на один і той самий partition, що критично для ordering guarantees при шардингу.

## Поточний стан

### Partition keys

| Webhook | Поточний key | Код |
|---------|--------------|-----|
| `PaymentWebhook` | `webhook.OrderId` | `async.go:25` |
| `ChargebackWebhook` | `webhook.OrderID` | `async.go:33` |

### Наявність user_id

| Struct | Поле user_id | JSON tag |
|--------|--------------|----------|
| `PaymentWebhook` | `UserId string` | `"user_id"` |
| `ChargebackWebhook` | **відсутнє** | — |

### Naming conventions (важливо!)

```go
// payment_entity.go - camelCase
type PaymentWebhook struct {
    OrderId string `json:"order_id"`  // camelCase
    UserId  string `json:"user_id"`   // camelCase
}

// chargeback_entity.go - uppercase ID
type ChargebackWebhook struct {
    OrderID string `json:"order_id"`  // uppercase ID
    // UserID - буде uppercase ID для консистентності
}
```

## Архітектурні рішення

| Питання | Рішення | Чому |
|---------|---------|------|
| Naming для нового поля | `UserID` (uppercase) | Консистентність всередині `ChargebackWebhook` з `OrderID` |
| JSON tag | `"user_id"` | Відповідає існуючому формату webhook payloads |
| Логування user_id | Так | Traceability для debugging |

## Імплементація

### Крок 1: Додати UserID до ChargebackWebhook

**Файл:** `internal/domain/dispute/chargeback_entity.go`

```go
type ChargebackWebhook struct {
    ProviderEventID string           `json:"provider_event_id"`
    OrderID         string           `json:"order_id"`
    UserID          string           `json:"user_id"`  // NEW
    Status          ChargebackStatus `json:"status"`
    // ... решта полів
}
```

### Крок 2: Змінити partition key в AsyncProcessor

**Файл:** `internal/webhook/async.go`

```go
// До:
func (p *AsyncProcessor) ProcessOrderWebhook(ctx context.Context, webhook order.PaymentWebhook) error {
    envelope, err := messaging.NewEnvelope(webhook.OrderId, "order.webhook", webhook)
    // ...
}

// Після:
func (p *AsyncProcessor) ProcessOrderWebhook(ctx context.Context, webhook order.PaymentWebhook) error {
    envelope, err := messaging.NewEnvelope(webhook.UserId, "order.webhook", webhook)
    // ...
}
```

```go
// До:
func (p *AsyncProcessor) ProcessDisputeWebhook(ctx context.Context, webhook dispute.ChargebackWebhook) error {
    envelope, err := messaging.NewEnvelope(webhook.OrderID, "dispute.webhook", webhook)
    // ...
}

// Після:
func (p *AsyncProcessor) ProcessDisputeWebhook(ctx context.Context, webhook dispute.ChargebackWebhook) error {
    envelope, err := messaging.NewEnvelope(webhook.UserID, "dispute.webhook", webhook)
    // ...
}
```

### Крок 3: Оновити логування в DisputeMessageController

**Файл:** `internal/controller/message/dispute.go`

Додати `user_id` до логів:

```go
// Duplicate event
c.logger.Info("Duplicate dispute event ignored: event_id=%s user_id=%s order_id=%s provider_event_id=%s",
    env.EventID, webhook.UserID, webhook.OrderID, webhook.ProviderEventID)

// Error
c.logger.Error("Failed to process chargeback webhook: event_id=%s user_id=%s order_id=%s error=%v",
    env.EventID, webhook.UserID, webhook.OrderID, err)

// Success
c.logger.Info("Chargeback webhook processed: event_id=%s user_id=%s order_id=%s status=%s",
    env.EventID, webhook.UserID, webhook.OrderID, webhook.Status)
```

### Крок 4: Оновити інтеграційні тести

**Файл:** `integration-test/integration_test.go`

Додати `user_id` до всіх chargeback payloads:

```go
openChargeback := map[string]interface{}{
    "provider_event_id": "evt-1",
    "order_id":          orderID,
    "user_id":           "44444444-4444-4444-4444-444444444444",  // NEW - same as order's user
    "status":            "opened",
    // ...
}
```

**Важливо:** `user_id` у chargeback має відповідати `user_id` замовлення, для якого створюється dispute.

## Файли для модифікації

| Файл | Зміни |
|------|-------|
| `internal/domain/dispute/chargeback_entity.go` | Додати `UserID string` поле |
| `internal/webhook/async.go` | Змінити partition key на `user_id` |
| `internal/controller/message/dispute.go` | Додати `user_id` до логів |
| `integration-test/integration_test.go` | Додати `user_id` до chargeback payloads |

## Порядок імплементації

1. [ ] Додати `UserID` до `ChargebackWebhook`
2. [ ] Змінити partition key в `AsyncProcessor` (обидва методи)
3. [ ] Оновити логування в `DisputeMessageController`
4. [ ] Оновити інтеграційні тести — додати `user_id` до chargebacks
5. [ ] Запустити тести: `make test && make integration-test`

## Чому user_id як partition key?

При шардингу бази даних по `user_id`:
- Всі дані користувача на одному шарді
- Kafka partition key = `user_id` гарантує ordering per user
- Consumer може обробляти події в правильному порядку
- Cross-shard queries мінімізовані

```
User A events → Partition 0 → Consumer 0 → Shard A
User B events → Partition 1 → Consumer 1 → Shard B
```
