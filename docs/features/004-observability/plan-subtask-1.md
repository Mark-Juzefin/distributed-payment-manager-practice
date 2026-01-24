# План: Subtask 1 - Metrics Foundation

## Мета

Додати Prometheus інструментацію для HTTP handlers (latency, request count) та Kafka consumers (lag, processing time).

## Метрики

| Metric | Type | Labels | Опис |
|--------|------|--------|------|
| `dpm_http_request_duration_seconds` | Histogram | handler, method, status_code | HTTP latency (p50/p95/p99) |
| `dpm_http_requests_total` | Counter | handler, method, status_code | Request count |
| `dpm_kafka_consumer_lag` | Gauge | topic, consumer_group | Messages behind |
| `dpm_kafka_message_processing_duration_seconds` | Histogram | topic, consumer_group | Processing time |

## Структура пакету

```
pkg/metrics/
├── metrics.go    # Metric definitions + init()
├── gin.go        # HTTP middleware
└── kafka.go      # Lag collector
```

## Файли для модифікації

| Файл | Зміни |
|------|-------|
| `go.mod` | +prometheus/client_golang |
| `pkg/metrics/metrics.go` | **NEW** - definitions |
| `pkg/metrics/gin.go` | **NEW** - middleware |
| `pkg/metrics/kafka.go` | **NEW** - lag collector |
| `internal/api/gin_engine.go` | +metrics.GinMetrics() |
| `internal/api/router.go` | +/metrics endpoint |
| `internal/api/workers.go` | +lag collectors |
| `internal/api/messaging/middleware.go` | +WithMetrics |
| `internal/ingest/app.go` | +metrics.GinMetrics() |
| `internal/ingest/router.go` | +/metrics endpoint |

## Порядок імплементації

### 1. Dependency
```bash
go get github.com/prometheus/client_golang/prometheus
go get github.com/prometheus/client_golang/prometheus/promhttp
```

### 2. pkg/metrics/metrics.go
```go
package metrics

import "github.com/prometheus/client_golang/prometheus"

var (
    HTTPRequestDuration = prometheus.NewHistogramVec(
        prometheus.HistogramOpts{
            Namespace: "dpm",
            Subsystem: "http",
            Name:      "request_duration_seconds",
            Help:      "HTTP request latency in seconds",
            Buckets:   []float64{.005, .01, .025, .05, .1, .25, .5, 1, 2.5, 5, 10},
        },
        []string{"handler", "method", "status_code"},
    )

    HTTPRequestsTotal = prometheus.NewCounterVec(
        prometheus.CounterOpts{
            Namespace: "dpm",
            Subsystem: "http",
            Name:      "requests_total",
            Help:      "Total number of HTTP requests",
        },
        []string{"handler", "method", "status_code"},
    )

    KafkaConsumerLag = prometheus.NewGaugeVec(
        prometheus.GaugeOpts{
            Namespace: "dpm",
            Subsystem: "kafka",
            Name:      "consumer_lag",
            Help:      "Number of messages consumer is behind",
        },
        []string{"topic", "consumer_group"},
    )

    KafkaMessageProcessingDuration = prometheus.NewHistogramVec(
        prometheus.HistogramOpts{
            Namespace: "dpm",
            Subsystem: "kafka",
            Name:      "message_processing_duration_seconds",
            Help:      "Time to process a Kafka message",
            Buckets:   []float64{.001, .005, .01, .025, .05, .1, .25, .5, 1, 2.5},
        },
        []string{"topic", "consumer_group"},
    )
)

func init() {
    prometheus.MustRegister(
        HTTPRequestDuration,
        HTTPRequestsTotal,
        KafkaConsumerLag,
        KafkaMessageProcessingDuration,
    )
}
```

### 3. pkg/metrics/gin.go
```go
package metrics

import (
    "strconv"
    "time"
    "github.com/gin-gonic/gin"
)

func GinMetrics() gin.HandlerFunc {
    return func(c *gin.Context) {
        start := time.Now()
        c.Next()

        duration := time.Since(start).Seconds()
        status := strconv.Itoa(c.Writer.Status())
        handler := c.FullPath()
        if handler == "" {
            handler = "unknown"
        }

        HTTPRequestDuration.WithLabelValues(handler, c.Request.Method, status).Observe(duration)
        HTTPRequestsTotal.WithLabelValues(handler, c.Request.Method, status).Inc()
    }
}
```

### 4. pkg/metrics/kafka.go
```go
package metrics

import (
    "context"
    "time"
    "github.com/segmentio/kafka-go"
)

type StatsProvider interface {
    Stats() kafka.ReaderStats
}

func StartLagCollector(ctx context.Context, reader StatsProvider, topic, groupID string, interval time.Duration) {
    go func() {
        ticker := time.NewTicker(interval)
        defer ticker.Stop()
        for {
            select {
            case <-ctx.Done():
                return
            case <-ticker.C:
                stats := reader.Stats()
                KafkaConsumerLag.WithLabelValues(topic, groupID).Set(float64(stats.Lag))
            }
        }
    }()
}
```

### 5. internal/api/messaging/middleware.go - додати WithMetrics
```go
func WithMetrics(handler MessageHandler, topic, groupID string) MessageHandler {
    return func(ctx context.Context, key, value []byte) error {
        start := time.Now()
        err := handler(ctx, key, value)
        metrics.KafkaMessageProcessingDuration.WithLabelValues(topic, groupID).Observe(time.Since(start).Seconds())
        return err
    }
}
```

### 6. internal/api/gin_engine.go
```go
func NewGinEngine(l *logger.Logger) *gin.Engine {
    engine := gin.New()
    engine.Use(metrics.GinMetrics(), l.GinBodyLogger(), gin.Recovery())
    return engine
}
```

### 7. internal/api/router.go - додати /metrics
```go
import "github.com/prometheus/client_golang/prometheus/promhttp"

func (r *Router) SetUp(engine *gin.Engine) {
    engine.GET("/metrics", gin.WrapH(promhttp.Handler()))
    // ... existing routes
}
```

### 8. internal/api/workers.go - додати lag collectors + WithMetrics
```go
// After creating consumers, start lag collectors:
metrics.StartLagCollector(ctx, orderConsumer.Reader(), cfg.KafkaOrdersTopic, cfg.KafkaOrdersConsumerGroup, 10*time.Second)

// Wrap handlers with metrics (outermost):
orderHandler := messaging.WithMetrics(
    messaging.WithDLQ(...),
    cfg.KafkaOrdersTopic,
    cfg.KafkaOrdersConsumerGroup,
)
```

### 9. internal/ingest/app.go + router.go
Аналогічні зміни для HTTP metrics (без Kafka).

## Verification

```bash
# 1. Start API
make run-dev

# 2. Generate traffic
curl http://localhost:3000/health
curl http://localhost:3000/orders

# 3. Check metrics
curl http://localhost:3000/metrics | grep dpm_

# Expected:
# dpm_http_requests_total{handler="/health",method="GET",status_code="200"} 1
# dpm_http_request_duration_seconds_bucket{...}
```

## Notes

- Metrics middleware йде ПЕРЕД logger middleware (щоб виміряти повний час)
- `c.FullPath()` повертає route pattern (`/orders/:order_id`), не actual path — це важливо для cardinality
- Kafka lag collector працює в background goroutine з 10s interval
