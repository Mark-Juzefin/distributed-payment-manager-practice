# План: Correlation IDs

## Мета

Додати наскрізний Correlation ID для трейсингу запитів через всі сервіси та компоненти системи.

## Поточний стан

- Логування через Zerolog (`pkg/logger/`) без контексту
- HTTP middleware: metrics → body logger → recovery
- Kafka messages використовують `Envelope` з `EventID`, але headers не використовуються
- Context передається через всі шари, але не містить correlation data

## Архітектурні рішення

| Питання | Рішення | Чому |
|---------|---------|------|
| Де генерувати ID? | На вході в Ingest (або приймати від клієнта) | Один ID на весь lifecycle запиту |
| Формат ID | UUID v4 | Унікальність, стандарт |
| Header name | `X-Correlation-ID` | Індустріальний стандарт |
| Передача через Kafka | Kafka headers | Не змінює payload, стандартний підхід |
| Інтеграція з логами | Context-aware методи | Zero boilerplate, автоматичне включення |

## Структура пакетів

```
pkg/
├── correlation/           # NEW: Correlation ID utilities
│   └── correlation.go     # Context key, Get/Set functions
└── logger/
    └── logger.go          # Add *Ctx methods

internal/
├── api/
│   ├── gin_engine.go      # Add correlation middleware
│   ├── external/kafka/
│   │   ├── publisher.go   # Add correlation header
│   │   └── consumer.go    # Extract correlation header
│   └── messaging/
│       └── middleware.go  # Add WithCorrelation middleware
└── ingest/
    └── app.go             # Add correlation middleware
```

## Імплементація

### Крок 1: Correlation Package

Створити `pkg/correlation/correlation.go`:

```go
package correlation

import (
    "context"

    "github.com/google/uuid"
)

// HeaderName is the HTTP header for correlation ID
const HeaderName = "X-Correlation-ID"

// KafkaHeaderName is the Kafka header for correlation ID
const KafkaHeaderName = "X-Correlation-ID"

type contextKey struct{}

// FromContext extracts correlation ID from context.
// Returns empty string if not present.
func FromContext(ctx context.Context) string {
    if id, ok := ctx.Value(contextKey{}).(string); ok {
        return id
    }
    return ""
}

// WithCorrelationID returns a new context with correlation ID.
func WithCorrelationID(ctx context.Context, id string) context.Context {
    return context.WithValue(ctx, contextKey{}, id)
}

// NewID generates a new correlation ID (UUID v4).
func NewID() string {
    return uuid.New().String()
}
```

### Крок 2: Context-Aware Logger

Додати методи в `pkg/logger/logger.go`:

```go
// InfoCtx logs info message with correlation ID from context
func (l *Logger) InfoCtx(ctx context.Context, format string, args ...any) {
    msg := fmt.Sprintf(format, args...)
    event := l.logger.Info()
    if corrID := correlation.FromContext(ctx); corrID != "" {
        event = event.Str("correlation_id", corrID)
    }
    event.Msg(msg)
}

// ErrorCtx logs error message with correlation ID from context
func (l *Logger) ErrorCtx(ctx context.Context, format string, args ...any) {
    msg := fmt.Sprintf(format, args...)
    event := l.logger.Error()
    if corrID := correlation.FromContext(ctx); corrID != "" {
        event = event.Str("correlation_id", corrID)
    }
    event.Msg(msg)
}

// WarnCtx, DebugCtx - аналогічно
```

### Крок 3: HTTP Middleware

Додати в `pkg/logger/gin.go` або окремий файл:

```go
// CorrelationMiddleware extracts or generates X-Correlation-ID
func CorrelationMiddleware() gin.HandlerFunc {
    return func(c *gin.Context) {
        corrID := c.GetHeader(correlation.HeaderName)
        if corrID == "" {
            corrID = correlation.NewID()
        }

        // Store in gin context (accessible via c.Value())
        ctx := correlation.WithCorrelationID(c.Request.Context(), corrID)
        c.Request = c.Request.WithContext(ctx)

        // Also store in gin keys for easy access
        c.Set("correlation_id", corrID)

        // Add to response header
        c.Header(correlation.HeaderName, corrID)

        c.Next()
    }
}
```

Оновити `internal/api/gin_engine.go`:

```go
engine.Use(
    logger.CorrelationMiddleware(),  // FIRST: before any logging
    metrics.GinMiddleware(),
    l.GinBodyLogger(),
    gin.Recovery(),
)
```

### Крок 4: Kafka Publisher

Оновити `internal/api/external/kafka/publisher.go`:

```go
func (p *Publisher) Publish(ctx context.Context, topic string, msg messaging.Envelope) error {
    value, err := json.Marshal(msg)
    if err != nil {
        return fmt.Errorf("marshal message: %w", err)
    }

    headers := []kafka.Header{}
    if corrID := correlation.FromContext(ctx); corrID != "" {
        headers = append(headers, kafka.Header{
            Key:   correlation.KafkaHeaderName,
            Value: []byte(corrID),
        })
    }

    err = p.writer.WriteMessages(ctx, kafka.Message{
        Key:     []byte(msg.Key),
        Value:   value,
        Headers: headers,
    })
    // ...
}
```

### Крок 5: Kafka Consumer Middleware

Додати в `internal/api/messaging/middleware.go`:

```go
// WithCorrelation extracts correlation ID from Kafka headers and injects into context
func WithCorrelation(next Handler) Handler {
    return HandlerFunc(func(ctx context.Context, msg kafka.Message) error {
        corrID := ""
        for _, h := range msg.Headers {
            if h.Key == correlation.KafkaHeaderName {
                corrID = string(h.Value)
                break
            }
        }

        if corrID == "" {
            corrID = correlation.NewID() // fallback for messages without correlation
        }

        ctx = correlation.WithCorrelationID(ctx, corrID)
        return next.HandleMessage(ctx, msg)
    })
}
```

**Важливо:** Потрібно змінити сигнатуру Handler щоб передавати `kafka.Message` замість `key, value []byte`. Це дозволить доступ до headers.

### Крок 6: Оновити Handler Interface

Поточна сигнатура:
```go
type Handler interface {
    HandleMessage(ctx context.Context, key, value []byte) error
}
```

Нова сигнатура (для доступу до headers):
```go
type Handler interface {
    HandleMessage(ctx context.Context, msg kafka.Message) error
}
```

Це breaking change для consumers, але дає доступ до:
- `msg.Key`
- `msg.Value`
- `msg.Headers`
- `msg.Topic`
- `msg.Partition`
- `msg.Offset`

### Крок 7: Ingest Service

Додати той самий middleware в `internal/ingest/app.go`:

```go
engine.Use(
    logger.CorrelationMiddleware(),  // Generate correlation ID for webhooks
    metrics.GinMiddleware(),
    gin.Recovery(),
)
```

### Крок 8: Оновити GinBodyLogger

Додати correlation ID до логів body logger в `pkg/logger/gin.go`:

```go
func (l *Logger) GinBodyLogger() gin.HandlerFunc {
    return func(c *gin.Context) {
        // ... existing code ...

        // After c.Next()
        corrID, _ := c.Get("correlation_id")
        l.logger.Info().
            Str("correlation_id", corrID.(string)).  // ADD THIS
            Str("method", c.Request.Method).
            // ... rest of fields
            Msg("request completed")
    }
}
```

## Файли для модифікації

| Файл | Зміни |
|------|-------|
| `pkg/correlation/correlation.go` | **NEW** - context utilities |
| `pkg/logger/logger.go` | Add `*Ctx` methods |
| `pkg/logger/gin.go` | Add `CorrelationMiddleware()`, update `GinBodyLogger()` |
| `internal/api/gin_engine.go` | Add correlation middleware to chain |
| `internal/ingest/app.go` | Add correlation middleware to chain |
| `internal/api/external/kafka/publisher.go` | Add correlation header |
| `internal/api/external/kafka/consumer.go` | Pass full `kafka.Message` to handler |
| `internal/api/messaging/types.go` | Update `Handler` interface |
| `internal/api/messaging/middleware.go` | Add `WithCorrelation()`, update other middlewares |
| `internal/api/consumers/order.go` | Update to new Handler signature |
| `internal/api/consumers/dispute.go` | Update to new Handler signature |
| `internal/api/workers.go` | Add `WithCorrelation` to middleware chain |

## Порядок імплементації

1. **Створити `pkg/correlation/`** - базовий пакет без залежностей
2. **Оновити Logger** - додати `*Ctx` методи
3. **HTTP Middleware** - `CorrelationMiddleware()` в `pkg/logger/gin.go`
4. **Підключити до API** - `internal/api/gin_engine.go`
5. **Підключити до Ingest** - `internal/ingest/app.go`
6. **Оновити Handler interface** - `internal/api/messaging/types.go`
7. **Оновити Kafka consumer** - `internal/api/external/kafka/consumer.go`
8. **Kafka middleware** - `WithCorrelation()` в `internal/api/messaging/middleware.go`
9. **Оновити consumer handlers** - order.go, dispute.go
10. **Оновити Publisher** - додати headers
11. **Оновити workers.go** - middleware chain
12. **Оновити GinBodyLogger** - включити correlation_id

## Тестування

### Unit Tests

- `pkg/correlation/correlation_test.go` - context operations
- `pkg/logger/logger_test.go` - verify `*Ctx` methods include correlation_id

### Integration Tests

- Verify correlation ID propagates through HTTP → Kafka → Consumer flow
- Verify logs contain correlation_id field

### Manual Testing

```bash
# Send request with correlation ID
curl -H "X-Correlation-ID: test-123" http://localhost:8081/webhooks/payments/orders -d '{...}'

# Check logs contain correlation_id: test-123
# Check response has X-Correlation-ID: test-123 header
```

## Міграція існуючого коду

Після імплементації можна поступово мігрувати існуючі log calls:

```go
// Before
l.Info("processing order: %s", orderID)

// After (where context available)
l.InfoCtx(ctx, "processing order: %s", orderID)
```

Це не breaking change - старі методи залишаються працювати.
