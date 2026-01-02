# Notes: Kafka Ingestion (Feature 001)

## Key Files

| File | Purpose |
|------|---------|
| `internal/messaging/types.go` | Envelope, Publisher, Worker interfaces |
| `internal/external/kafka/consumer.go` | Kafka consumer (segmentio/kafka-go) |
| `internal/external/kafka/publisher.go` | Kafka publisher with Hash balancer |
| `internal/external/kafka/dlq_publisher.go` | Dead Letter Queue publisher |
| `internal/messaging/middleware.go` | Retry + DLQ middleware |
| `internal/messaging/runner.go` | Worker lifecycle manager |
| `internal/webhook/processor.go` | Processor interface |
| `internal/webhook/async.go` | AsyncProcessor (HTTP → Kafka) |
| `internal/webhook/sync.go` | SyncProcessor (HTTP → Service) |
| `internal/controller/message/order.go` | Order message handler |
| `internal/controller/message/dispute.go` | Dispute message handler |
| `internal/app/workers.go` | Consumer startup |

---

## Основні сутності

### Envelope (internal/messaging/types.go)

Обгортка повідомлення для Kafka:
```go
type Envelope struct {
    EventID   string          // UUID для трейсингу
    Key       string          // Partition key (user_id)
    Type      string          // "order.webhook", "dispute.webhook"
    Payload   json.RawMessage // Serialized webhook
    Timestamp time.Time
}
```

### Publisher interface

```go
type Publisher interface {
    Publish(ctx context.Context, envelope Envelope) error
    Close() error
}
```
- `kafka.Publisher` — імплементація з Hash balancer для partition by key

### Worker interface

```go
type Worker interface {
    Start(ctx context.Context, handler MessageHandler) error
    Close() error
}
```
- `kafka.Consumer` — blocking consume loop з manual commit

### MessageHandler

```go
type MessageHandler func(ctx context.Context, key, value []byte) error
```
- Якщо return nil → commit offset
- Якщо return error → offset не commit (re-delivery при перезапуску)

---

## Sync/Kafka Mode

### WEBHOOK_MODE env variable

**`sync`** (default):
```
HTTP POST /webhooks → OrderHandler → SyncProcessor → OrderService
                                                   ↓
                                            Direct call, wait for result
```

**`kafka`**:
```
HTTP POST /webhooks → OrderHandler → AsyncProcessor → Kafka Publisher
                                                    ↓
                                            Return immediately (202 Accepted)

[Background consumer]
Kafka → Consumer → MessageHandler → OrderService
                                  ↓
                            Process async
```

### Processor interface (internal/webhook/processor.go)

```go
type Processor interface {
    ProcessOrderWebhook(ctx, webhook) error
    ProcessDisputeWebhook(ctx, webhook) error
}
```

Обидва режими implement цей interface → handlers не знають про режим.

### Перемикання в app.go

```go
if cfg.WebhookMode == "kafka" {
    processor = webhook.NewAsyncProcessor(orderPub, disputePub)
    StartWorkers(ctx, cfg, orderService, disputeService)
} else {
    processor = webhook.NewSyncProcessor(orderService, disputeService)
}
```

---

## Retry/DLQ механіка

### RetryConfig (internal/messaging/middleware.go)

```go
DefaultRetryConfig() = {
    MaxAttempts:    3,
    InitialBackoff: 100ms,  // → 200ms → 400ms (exponential)
    MaxBackoff:     5s,
    Jitter:         100ms,  // Random 0-100ms to avoid thundering herd
}
```

### Middleware chain

```go
handler := WithDLQ(
    WithRetry(controller.HandleMessage, DefaultRetryConfig()),
    dlqPublisher,
)
```

**WithRetry:**
1. Call handler
2. If success → return nil
3. If error → sleep(backoff + jitter) → retry
4. After MaxAttempts → return ErrMaxRetriesExceeded

**WithDLQ:**
1. Call inner handler (WithRetry)
2. If success → return nil (commit offset)
3. If error → publish to DLQ topic → return nil (commit anyway!)

### Чому commit навіть при failure?

Без цього poison message заблокує partition назавжди. DLQ дозволяє:
- Consumer прогресує
- Failed messages зберігаються для аналізу
- Можна reprocess з DLQ пізніше

---

## Message Flow

### Kafka mode (повний шлях):

```
1. HTTP POST /webhooks/payments/orders {order_id, user_id, status...}
2. orderHandler.Webhook() парсить JSON
3. processor.ProcessOrderWebhook(webhook)
4. asyncProcessor створює Envelope:
   - EventID: new UUID
   - Key: webhook.UserId (partition key!)
   - Type: "order.webhook"
   - Payload: serialized webhook
5. publisher.Publish() → kafka.Writer.WriteMessages()
6. HTTP 200 (published, not processed yet)

[Consumer goroutine]
7. consumer.Start() → reader.FetchMessage()
8. WithDLQ → WithRetry → orderController.HandleMessage()
9. Unmarshal Envelope.Payload → webhook
10. orderService.ProcessPaymentWebhook(webhook)
11. orderRepo.Update() + orderEventSink.CreateEvent()
12. Return nil → reader.CommitMessages()
13. Continue to next message
```

---

## Partition Key Design

### Чому user_id, а не order_id?

| Аспект | order_id | user_id |
|--------|----------|---------|
| Ordering | Per order (занадто granular) | Per user (зберігає causality) |
| Sharding | Не відповідає DB shards | Ідеально для hash(user_id) sharding |
| Load | Hot orders = hot partition | Рівномірний розподіл |
| Queries | Cross-shard для user data | Всі дані user на одному shard |

### Приклад

```
User A: Order1, Order2, Order3 → Partition 0
User B: Order4, Order5         → Partition 1

Kafka гарантує ordering ВСЕРЕДИНІ partition:
- User A events: Order1 → Order2 → Order3 (guaranteed order)
- User B events: паралельно, не блокують User A
```

### Код (internal/webhook/async.go)

```go
// Partition key = user_id
envelope := messaging.NewEnvelope(webhook.UserId, "order.webhook", webhook)
```

---

## Idempotency

### Проблема

Kafka забезпечує at-least-once delivery. Consumer може отримати duplicate messages:
- Network issues during commit
- Consumer restart before commit
- Kafka rebalancing

### Рішення

**1. UNIQUE constraint в БД:**
```sql
-- order_events
UNIQUE (order_id, provider_event_id)

-- dispute_events (partitioned table!)
UNIQUE (dispute_id, provider_event_id, created_at)
                                       ↑ partition key required!
```

**2. Handler logic (internal/controller/message/order.go):**
```go
err := service.ProcessPaymentWebhook(ctx, webhook)
if errors.Is(err, order.ErrEventAlreadyStored) {
    logger.Info("Duplicate event ignored", "event_id", env.EventID)
    return nil  // Success! Commit offset
}
```

**3. Result:**
- Duplicate message → DB rejects → handler returns nil → offset commits
- Consumer прогресує, немає infinite retry loop

---

## Твої думки

<!--
Тут можеш додати свої нотатки:
- Що було складним
- Що можна покращити
- Ідеї для майбутнього
-->
