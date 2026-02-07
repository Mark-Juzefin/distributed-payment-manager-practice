# План: Kafka Metrics

## Мета

Додати Prometheus метрики для Kafka consumer:
1. **Consumer lag** — скільки повідомлень consumer позаду committed offset
2. **Processing duration** — час обробки кожного повідомлення

## Поточний стан

- Kafka consumers реалізовані в `internal/api/external/kafka/consumer.go`
- Є middleware pattern для handlers: `WithRetry`, `WithDLQ`
- HTTP метрики в `pkg/metrics/` з реєстрацією в `Registry`
- `kafka-go` Reader має метод `Stats()` який повертає статистику включно з lag

## Архітектурні рішення

| Питання | Варіанти | Рішення | Чому |
|---------|----------|---------|------|
| Де збирати processing duration? | A) В consumer.go B) Middleware | **B) Middleware** | Слідуємо існуючому pattern (retry/dlq), separation of concerns |
| Як збирати consumer lag? | A) Background goroutine B) Custom Prometheus Collector | **A) Background goroutine** | Простіше для імплементації, не потребує глобального реєстру consumers |
| Які labels для processing duration? | topic, consumer_group, status | `topic`, `consumer_group`, `status` | Достатньо для drill-down без cardinality explosion |
| Buckets для histogram | Стандартні vs кастомні | Кастомні: `.001, .005, .01, .025, .05, .1, .25, .5, 1, 2.5` | Kafka повідомлення обробляються швидше ніж HTTP requests |

## Метрики

```prometheus
# Histogram для processing duration
dpm_kafka_message_processing_duration_seconds{topic, consumer_group, status}

# Counter для throughput
dpm_kafka_messages_processed_total{topic, consumer_group, status}

# Gauge для consumer lag
dpm_kafka_consumer_lag{topic, consumer_group, partition}
```

**Status values:** `success`, `error`, `dlq`

## Структура файлів

```
pkg/metrics/
├── registry.go     # (існує)
├── http.go         # (існує)
├── middleware.go   # (існує - Gin middleware)
└── kafka.go        # НОВИЙ - Kafka metric definitions

internal/api/messaging/
├── middleware.go   # (існує) - додаємо WithMetrics
└── runner.go       # (існує) - додаємо lag collection
```

## Імплементація

### 1. `pkg/metrics/kafka.go` — Metric definitions

```go
package metrics

import "github.com/prometheus/client_golang/prometheus"

var (
    KafkaProcessingDuration = prometheus.NewHistogramVec(
        prometheus.HistogramOpts{
            Namespace: "dpm",
            Subsystem: "kafka",
            Name:      "message_processing_duration_seconds",
            Help:      "Kafka message processing duration in seconds",
            Buckets:   []float64{.001, .005, .01, .025, .05, .1, .25, .5, 1, 2.5},
        },
        []string{"topic", "consumer_group", "status"},
    )

    KafkaMessagesProcessed = prometheus.NewCounterVec(
        prometheus.CounterOpts{
            Namespace: "dpm",
            Subsystem: "kafka",
            Name:      "messages_processed_total",
            Help:      "Total number of Kafka messages processed",
        },
        []string{"topic", "consumer_group", "status"},
    )

    KafkaConsumerLag = prometheus.NewGaugeVec(
        prometheus.GaugeOpts{
            Namespace: "dpm",
            Subsystem: "kafka",
            Name:      "consumer_lag",
            Help:      "Kafka consumer lag (messages behind)",
        },
        []string{"topic", "consumer_group", "partition"},
    )
)

func init() {
    Registry.MustRegister(KafkaProcessingDuration, KafkaMessagesProcessed, KafkaConsumerLag)
}
```

### 2. `internal/api/messaging/middleware.go` — WithMetrics middleware

```go
// WithMetrics wraps handler to record processing duration and message counts.
func WithMetrics(topic, consumerGroup string, handler MessageHandler) MessageHandler {
    return func(ctx context.Context, key, value []byte) error {
        start := time.Now()

        err := handler(ctx, key, value)

        duration := time.Since(start).Seconds()
        status := "success"
        if err != nil {
            status = "error"
        }

        metrics.KafkaProcessingDuration.WithLabelValues(topic, consumerGroup, status).Observe(duration)
        metrics.KafkaMessagesProcessed.WithLabelValues(topic, consumerGroup, status).Inc()

        return err
    }
}
```

**Middleware order:**
```
WithMetrics → WithRetry → WithDLQ → Handler
     ↑ Measures total processing time including retries
```

### 3. `internal/api/external/kafka/consumer.go` — Add Stats method

```go
// Stats returns current reader statistics.
func (c *Consumer) Stats() kafka.ReaderStats {
    return c.reader.Stats()
}
```

### 4. `internal/api/messaging/runner.go` — Lag collection goroutine

Додаємо interface для workers що підтримують stats:

```go
// StatsProvider provides Kafka consumer statistics.
type StatsProvider interface {
    Stats() kafka.ReaderStats
}

// Runner starts lag collection goroutine
func (r *Runner) Start(ctx context.Context) error {
    // Start lag collector if workers support stats
    for _, w := range r.workers {
        if sp, ok := w.(StatsProvider); ok {
            go r.collectLag(ctx, sp)
        }
    }

    // ... existing worker startup code
}

func (r *Runner) collectLag(ctx context.Context, sp StatsProvider) {
    ticker := time.NewTicker(15 * time.Second)
    defer ticker.Stop()

    for {
        select {
        case <-ctx.Done():
            return
        case <-ticker.C:
            stats := sp.Stats()
            metrics.KafkaConsumerLag.WithLabelValues(
                stats.Topic,
                stats.GroupID,
                "aggregate",
            ).Set(float64(stats.Lag))
        }
    }
}
```

### 5. `internal/api/workers.go` — Wire metrics middleware

```go
// Before (current):
orderHandler := messaging.WithRetry(messaging.WithDLQ(...))

// After:
orderHandler := messaging.WithMetrics(
    cfg.KafkaOrdersTopic,
    cfg.KafkaOrdersConsumerGroup,
    messaging.WithRetry(messaging.WithDLQ(...)),
)
```

## Файли для модифікації

| Файл | Зміни |
|------|-------|
| `pkg/metrics/kafka.go` | **NEW** — Kafka metric definitions |
| `internal/api/messaging/middleware.go` | Додати `WithMetrics` middleware |
| `internal/api/messaging/runner.go` | Додати lag collection goroutine |
| `internal/api/external/kafka/consumer.go` | Додати `Stats()` method |
| `internal/api/workers.go` | Wire `WithMetrics` middleware |

## Порядок імплементації

1. [x] Створити `pkg/metrics/kafka.go` з metric definitions
2. [x] Додати `Stats()` та `LagStats()` методи до `Consumer`
3. [x] Додати `WithMetrics` middleware в `messaging/middleware.go`
4. [x] Додати lag collection в `Runner`
5. [x] Підключити middleware в `workers.go`
6. [x] Тест: `make run-kafka` + `curl localhost:3000/metrics | grep dpm_kafka`

## Альтернативи (що не обрали)

**Custom Prometheus Collector для lag:**
- Плюси: No background goroutine, fresh values on scrape
- Мінуси: Потребує глобального реєстру consumers, складніше тестувати, coupling

**Instrument в consumer.go напряму:**
- Плюси: Все в одному місці
- Мінуси: Порушує separation of concerns, consumer не знає про metrics
