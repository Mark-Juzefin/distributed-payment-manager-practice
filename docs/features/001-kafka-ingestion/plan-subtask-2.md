# План: Конфігурабельний WebhookProcessor

## Мета
Додати можливість вибору через env змінну: обробляти webhooks напряму (sync) чи через Kafka (async).

## Архітектура

```
                    ┌─────────────────────────────┐
                    │    WebhookProcessor         │
                    │  (interface)                │
                    ├─────────────────────────────┤
                    │ ProcessOrderWebhook()       │
                    │ ProcessDisputeWebhook()     │
                    └──────────────┬──────────────┘
                                   │
              ┌────────────────────┼────────────────────┐
              │                                         │
              ▼                                         ▼
┌─────────────────────────┐             ┌─────────────────────────┐
│  SyncWebhookProcessor   │             │  AsyncWebhookProcessor  │
│  (calls services)       │             │  (publishes to Kafka)   │
└─────────────────────────┘             └─────────────────────────┘
```

**Handler використовує інтерфейс:**
```
OrderHandler.Webhook() → processor.ProcessOrderWebhook()
ChargebackHandler.Webhook() → processor.ProcessDisputeWebhook()
```

## Новий інтерфейс

**Файл:** `internal/webhook/processor.go`

```go
package webhook

type Processor interface {
    ProcessOrderWebhook(ctx context.Context, webhook order.PaymentWebhook) error
    ProcessDisputeWebhook(ctx context.Context, webhook dispute.ChargebackWebhook) error
}
```

## Реалізація 1: SyncProcessor

**Файл:** `internal/webhook/sync.go`

```go
type SyncProcessor struct {
    orderService   *order.OrderService
    disputeService *dispute.DisputeService
}

func (p *SyncProcessor) ProcessOrderWebhook(ctx context.Context, webhook order.PaymentWebhook) error {
    return p.orderService.ProcessPaymentWebhook(ctx, webhook)
}

func (p *SyncProcessor) ProcessDisputeWebhook(ctx context.Context, webhook dispute.ChargebackWebhook) error {
    return p.disputeService.ProcessChargeback(ctx, webhook)
}
```

## Реалізація 2: AsyncProcessor

**Файл:** `internal/webhook/async.go`

```go
type AsyncProcessor struct {
    orderPublisher   messaging.Publisher
    disputePublisher messaging.Publisher
}

func (p *AsyncProcessor) ProcessOrderWebhook(ctx context.Context, webhook order.PaymentWebhook) error {
    envelope, err := messaging.NewEnvelope(webhook.OrderId, "order.webhook", webhook)
    if err != nil {
        return err
    }
    return p.orderPublisher.Publish(ctx, envelope)
}

func (p *AsyncProcessor) ProcessDisputeWebhook(ctx context.Context, webhook dispute.ChargebackWebhook) error {
    envelope, err := messaging.NewEnvelope(webhook.OrderID, "dispute.webhook", webhook)
    if err != nil {
        return err
    }
    return p.disputePublisher.Publish(ctx, envelope)
}
```

## Зміни в Handlers

**OrderHandler:**
```go
type OrderHandler struct {
    service   *order.OrderService  // для інших методів (Get, Filter, etc.)
    processor webhook.Processor
}

func (h *OrderHandler) Webhook(c *gin.Context) {
    // ...bind event...
    if err := h.processor.ProcessOrderWebhook(c.Request.Context(), event); err != nil {
        // handle error
    }
    c.Status(http.StatusAccepted)
}
```

- Видалити `WebhookSync()` метод
- `Webhook()` використовує `processor`

**ChargebackHandler:** аналогічно

## Config

**Файл:** `config/config.go`
```go
WebhookMode string `env:"WEBHOOK_MODE" envDefault:"sync"` // "sync" | "kafka"
```

## App wiring

**Файл:** `internal/app/app.go`
```go
// Вибір processor на основі конфігурації
var processor webhook.Processor
if cfg.WebhookMode == "kafka" {
    orderPublisher := kafka.NewPublisher(...)
    disputePublisher := kafka.NewPublisher(...)
    processor = webhook.NewAsyncProcessor(orderPublisher, disputePublisher)

    // Запустити consumers тільки в kafka режимі
    StartWorkers(ctx, l, cfg, orderService, disputeService)
} else {
    processor = webhook.NewSyncProcessor(orderService, disputeService)
}

orderHandler := handlers.NewOrderHandler(orderService, processor)
chargebackHandler := handlers.NewChargebackHandler(disputeService, processor)
```

## Cleanup в Services

Видалити з services методи `QueuePaymentWebhook()` та `QueueChargebackWebhook()` - вони більше не потрібні, бо ця логіка тепер в `AsyncProcessor`.

Також видалити `publisher` поле з services - вони його більше не використовують.

## Файли для модифікації

| Файл | Зміни |
|------|-------|
| `internal/webhook/processor.go` | NEW: інтерфейс Processor |
| `internal/webhook/sync.go` | NEW: SyncProcessor |
| `internal/webhook/async.go` | NEW: AsyncProcessor |
| `config/config.go` | +WebhookMode |
| `.env.example` | +WEBHOOK_MODE |
| `internal/controller/rest/handlers/order.go` | +processor, -WebhookSync() |
| `internal/controller/rest/handlers/chargeback.go` | +processor, -WebhookSync() |
| `internal/domain/order/service.go` | -publisher, -QueuePaymentWebhook() |
| `internal/domain/dispute/service.go` | -publisher, -QueueChargebackWebhook() |
| `internal/app/app.go` | вибір processor на основі config |

## Порядок імплементації

1. Створити `internal/webhook/` пакет з інтерфейсом та двома реалізаціями
2. Оновити config + .env.example
3. Оновити handlers (додати processor, видалити WebhookSync)
4. Очистити services (видалити publisher та Queue* методи)
5. Оновити app.go (вибір processor)
