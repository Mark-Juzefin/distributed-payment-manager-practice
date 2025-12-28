# План: Kafka Webhook Processing

## Мета

Замінити синхронну обробку webhooks на async через Kafka:
```
HTTP Webhook → Handler → Publisher → Kafka → Consumer → Service → DB
```

Замість:
```
HTTP Webhook → Handler → Service → DB (sync, блокує HTTP response)
```

---

## Поточний стан

**Endpoints:**
- `POST /webhooks/payments/orders` → `OrderHandler.Webhook` → `OrderService.ProcessPaymentWebhook`
- `POST /webhooks/payments/chargebacks` → `ChargebackHandler.Webhook` → `DisputeService.ProcessChargeback`

**Структури:**
- `PaymentWebhook` - має `UserId`, `OrderId`, `ProviderEventID`
- `ChargebackWebhook` - має `OrderID`, `ProviderEventID`, але **НЕ має UserId**

**Idempotency:** готова (UNIQUE constraints + `ErrEventAlreadyStored`)

---

## Архітектурні рішення

| Питання | Рішення | Чому |
|---------|---------|------|
| Partition key | `order_id` | Ordering per order, ChargebackWebhook не має user_id |
| Kafka | Redpanda | Легкий для dev, Kafka-compatible |
| Бібліотека | segmentio/kafka-go | Проста, перевірена |
| Envelope | З event_id (UUID) | Для tracing та дебагу |
| DLQ | Пізніше | Окрема підзадача |

---

## Структура пакетів

```
internal/
├── messaging/
│   ├── types.go              # Publisher, Worker, MessageHandler, Envelope
│   └── runner.go             # Runner з errgroup + panic recovery
│
├── external/
│   └── kafka/
│       ├── publisher.go      # Producer (segmentio/kafka-go)
│       └── consumer.go       # Consumer (Worker implementation)
│
├── controller/
│   ├── rest/handlers/        # Модифіковані - publish замість sync
│   │   ├── order.go          # → publisher.Publish()
│   │   └── chargeback.go     # → publisher.Publish()
│   │
│   └── message/              # NEW: Kafka message handlers
│       ├── order.go          # OrderMessageController
│       └── dispute.go        # DisputeMessageController
│
├── app/
│   ├── app.go                # Wiring + workers startup
│   └── workers.go            # NEW: RunWorkers function
```

---

## Імплементація

### 1. Messaging абстракції (`internal/messaging/`)

**types.go:**
```go
type Envelope struct {
    EventID   string          `json:"event_id"`   // UUID
    Key       string          `json:"key"`        // order_id (partition key)
    Type      string          `json:"type"`       // "order.webhook" | "dispute.webhook"
    Payload   json.RawMessage `json:"payload"`
    Timestamp time.Time       `json:"timestamp"`
}

type Publisher interface {
    Publish(ctx context.Context, envelope Envelope) error
    Close() error
}

type MessageHandler func(ctx context.Context, key, value []byte) error

type Worker interface {
    Start(ctx context.Context, handler MessageHandler) error
    Close() error
}
```

**runner.go:**
```go
type Runner struct {
    logger  *logger.Logger
    workers []Worker
    handler MessageHandler
}

func (r *Runner) Start(ctx context.Context) error {
    g, ctx := errgroup.WithContext(ctx)

    for i, w := range r.workers {
        i, w := i, w
        g.Go(func() error {
            defer func() {
                if rec := recover(); rec != nil {
                    r.logger.Error("Worker panic", "idx", i,
                        "panic", rec, "stack", string(debug.Stack()))
                }
                _ = w.Close()
            }()
            return w.Start(ctx, r.handler)
        })
    }
    return g.Wait()
}
```

### 2. Kafka implementation (`internal/external/kafka/`)

**publisher.go:**
```go
type Publisher struct {
    writer *kafka.Writer
    logger *logger.Logger
}

func (p *Publisher) Publish(ctx context.Context, env messaging.Envelope) error {
    value, _ := json.Marshal(env)
    return p.writer.WriteMessages(ctx, kafka.Message{
        Key:   []byte(env.Key),
        Value: value,
    })
}
```

**consumer.go:**
```go
type Consumer struct {
    reader *kafka.Reader
    logger *logger.Logger
}

func (c *Consumer) Start(ctx context.Context, handler messaging.MessageHandler) error {
    for {
        m, err := c.reader.FetchMessage(ctx)
        if err != nil {
            if errors.Is(err, context.Canceled) {
                return nil
            }
            return err
        }

        if err := handler(ctx, m.Key, m.Value); err != nil {
            c.logger.Error("Handler error", "error", err)
            continue  // skip, no commit (retry on restart)
        }

        if err := c.reader.CommitMessages(ctx, m); err != nil {
            return err
        }
    }
}
```

### 3. Message Controllers (`internal/controller/message/`)

**order.go:**
```go
type OrderMessageController struct {
    logger  *logger.Logger
    service *order.OrderService
}

func (c *OrderMessageController) HandleMessage(ctx context.Context, key, value []byte) error {
    var env messaging.Envelope
    if err := json.Unmarshal(value, &env); err != nil {
        return fmt.Errorf("unmarshal envelope: %w", err)
    }

    var webhook order.PaymentWebhook
    if err := json.Unmarshal(env.Payload, &webhook); err != nil {
        return fmt.Errorf("unmarshal webhook: %w", err)
    }

    return c.service.ProcessPaymentWebhook(ctx, webhook)
}
```

**dispute.go:** аналогічно для `DisputeService.ProcessChargeback`

### 4. Модифікація Handlers

**order.go (before):**
```go
func (h *OrderHandler) Webhook(c *gin.Context) {
    err := h.service.ProcessPaymentWebhook(c, event)  // sync
}
```

**order.go (after):**
```go
func (h *OrderHandler) Webhook(c *gin.Context) {
    envelope := messaging.NewEnvelope(event.OrderId, "order.webhook", event)
    err := h.publisher.Publish(c, envelope)  // async to Kafka
    if err != nil {
        c.JSON(500, gin.H{"error": "failed to queue webhook"})
        return
    }
    c.Status(http.StatusAccepted)  // 202 - accepted for processing
}
```

### 5. App wiring (`internal/app/`)

**workers.go:**
```go
func RunWorkers(ctx context.Context, l *logger.Logger, cfg config.Config,
    orderService *order.OrderService, disputeService *dispute.DisputeService) {

    orderController := message.NewOrderMessageController(l, orderService)
    orderRunner := messaging.NewRunner(l,
        []messaging.Worker{
            kafka.NewConsumer(l, cfg.KafkaBrokers, cfg.KafkaOrdersTopic, cfg.KafkaOrdersConsumerGroup),
        },
        orderController.HandleMessage,
    )

    disputeController := message.NewDisputeMessageController(l, disputeService)
    disputeRunner := messaging.NewRunner(l,
        []messaging.Worker{
            kafka.NewConsumer(l, cfg.KafkaBrokers, cfg.KafkaDisputesTopic, cfg.KafkaDisputesConsumerGroup),
        },
        disputeController.HandleMessage,
    )

    go func() {
        if err := orderRunner.Start(ctx); err != nil {
            l.Error("Order runner failed", "error", err)
        }
    }()

    go func() {
        if err := disputeRunner.Start(ctx); err != nil {
            l.Error("Dispute runner failed", "error", err)
        }
    }()
}
```

**app.go:**
```go
func Run(cfg config.Config) {
    ctx, cancel := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
    defer cancel()

    // ... existing setup ...

    // Kafka publishers
    orderPublisher := kafka.NewPublisher(l, cfg.KafkaBrokers, cfg.KafkaOrdersTopic)
    disputePublisher := kafka.NewPublisher(l, cfg.KafkaBrokers, cfg.KafkaDisputesTopic)

    // Handlers with publishers
    orderHandler := handlers.NewOrderHandler(orderPublisher)
    chargebackHandler := handlers.NewChargebackHandler(disputePublisher)

    // Start workers
    RunWorkers(ctx, l, cfg, orderService, disputeService)

    // ... engine.Run() ...
}
```

### 6. Graceful Shutdown
- `signal.NotifyContext` для SIGINT/SIGTERM
- Context cancellation зупиняє consumers
- `defer cancel()` + `defer publisher.Close()`

---

## Файли для модифікації

| Файл | Зміни |
|------|-------|
| `docker-compose.yaml` | +Redpanda, +topics init |
| `config/config.go` | +Kafka fields |
| `.env.example` | +Kafka env vars |
| `internal/messaging/types.go` | NEW |
| `internal/messaging/runner.go` | NEW |
| `internal/external/kafka/publisher.go` | NEW |
| `internal/external/kafka/consumer.go` | NEW |
| `internal/controller/message/order.go` | NEW |
| `internal/controller/message/dispute.go` | NEW |
| `internal/controller/rest/handlers/order.go` | publish замість sync |
| `internal/controller/rest/handlers/chargeback.go` | publish замість sync |
| `internal/app/app.go` | wiring + graceful shutdown |
| `internal/app/workers.go` | NEW |
| `go.mod` | +segmentio/kafka-go |

---

## Порядок імплементації

1. docker-compose + config
2. messaging/types.go + runner.go
3. kafka/publisher.go + consumer.go
4. message controllers
5. modify handlers
6. app.go wiring
7. graceful shutdown
8. manual test
