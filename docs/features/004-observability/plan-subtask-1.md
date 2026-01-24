# План: Subtask 1 - HTTP Metrics

## Мета

Додати Prometheus інструментацію для HTTP handlers: latency histogram та request counter для обох сервісів (API та Ingest).

## Метрики

| Metric | Type | Labels | Опис |
|--------|------|--------|------|
| `dpm_http_request_duration_seconds` | Histogram | handler, method, status_code | HTTP latency (p50/p95/p99) |
| `dpm_http_requests_total` | Counter | handler, method, status_code | Request count |

## Поточний стан

- [x] Prometheus dependency додано (`go get github.com/prometheus/client_golang`)
- [x] `/metrics` endpoint в Ingest з Go/Process collectors
- [x] HTTP middleware для метрик
- [x] `/metrics` endpoint в API

## Архітектурні рішення

| Питання | Рішення | Чому |
|---------|---------|------|
| Global vs custom registry | Custom registry | Контроль над тим, що експортується |
| Middleware placement | Перед усіма handlers | Щоб виміряти повний час обробки |
| Handler label | `c.FullPath()` | Route pattern (`/orders/:id`), не actual path — контроль cardinality |

## Структура пакету

```
pkg/metrics/
├── registry.go   # Custom registry + init
├── http.go       # HTTP metrics definitions
└── middleware.go # Gin middleware
```

## Файли для модифікації

| Файл | Зміни |
|------|-------|
| `pkg/metrics/registry.go` | **NEW** - Custom registry з Go/Process collectors |
| `pkg/metrics/http.go` | **NEW** - HTTP metric definitions |
| `pkg/metrics/middleware.go` | **NEW** - Gin middleware |
| `internal/ingest/app.go` | Використати shared registry + middleware |
| `internal/ingest/router.go` | Оновити `/metrics` endpoint |
| `internal/api/gin_engine.go` | Додати metrics middleware |
| `internal/api/router.go` | Додати `/metrics` endpoint |

## Порядок імплементації

### 1. pkg/metrics/registry.go

```go
package metrics

import (
    "github.com/prometheus/client_golang/prometheus"
    "github.com/prometheus/client_golang/prometheus/collectors"
)

// Registry is the custom Prometheus registry for DPM metrics.
var Registry = prometheus.NewRegistry()

func init() {
    Registry.MustRegister(
        collectors.NewGoCollector(),
        collectors.NewProcessCollector(collectors.ProcessCollectorOpts{}),
    )
}
```

### 2. pkg/metrics/http.go

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
)

func init() {
    Registry.MustRegister(HTTPRequestDuration, HTTPRequestsTotal)
}
```

### 3. pkg/metrics/middleware.go

```go
package metrics

import (
    "strconv"
    "time"

    "github.com/gin-gonic/gin"
)

// GinMiddleware returns Gin middleware that records HTTP metrics.
func GinMiddleware() gin.HandlerFunc {
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

### 4. Оновити internal/ingest/app.go

```go
// Replace local registry with shared one
import "TestTaskJustPay/pkg/metrics"

// In Run():
engine.Use(metrics.GinMiddleware(), gin.Recovery())
```

### 5. Оновити internal/ingest/router.go

```go
import (
    "TestTaskJustPay/pkg/metrics"
    "github.com/prometheus/client_golang/prometheus/promhttp"
)

func (r *Router) SetUp(engine *gin.Engine) {
    engine.GET("/metrics", gin.WrapH(promhttp.HandlerFor(metrics.Registry, promhttp.HandlerOpts{})))
    // ... existing routes
}
```

### 6. Оновити internal/api/gin_engine.go

```go
import "TestTaskJustPay/pkg/metrics"

func NewGinEngine(l *logger.Logger) *gin.Engine {
    engine := gin.New()
    engine.Use(metrics.GinMiddleware(), l.GinBodyLogger(), gin.Recovery())
    return engine
}
```

### 7. Оновити internal/api/router.go

```go
import (
    "TestTaskJustPay/pkg/metrics"
    "github.com/prometheus/client_golang/prometheus/promhttp"
)

func (r *Router) SetUp(engine *gin.Engine) {
    engine.GET("/metrics", gin.WrapH(promhttp.HandlerFor(metrics.Registry, promhttp.HandlerOpts{})))
    // ... existing routes
}
```

## Verification

```bash
# 1. Start API
make run-dev

# 2. Generate traffic
make test-webhook
curl http://localhost:3000/health
curl http://localhost:3000/orders

# 3. Check metrics
curl -s http://localhost:3000/metrics | grep dpm_http

# Expected output:
# dpm_http_requests_total{handler="/health",method="GET",status_code="200"} 1
# dpm_http_request_duration_seconds_bucket{handler="/health",method="GET",status_code="200",le="0.005"} 1
# ...
```

## Notes

- Metrics middleware йде ПЕРЕД logger middleware (щоб виміряти повний час включно з logging)
- `c.FullPath()` повертає route pattern (`/orders/:order_id`), не actual path — це важливо для cardinality
- Custom registry дає контроль над експортованими метриками
