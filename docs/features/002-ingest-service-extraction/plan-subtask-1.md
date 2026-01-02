# План: Separate Ingest Service Binary

## Мета

Розділити монолітний бінарник на два незалежні сервіси:
- **Ingest Service** - легкий HTTP → Kafka gateway (приймає webhooks, публікує в топіки)
- **API Service** - основний сервіс з domain logic, Kafka consumers, manual operations, read endpoints

Зберегти "complex opt-in" філософію: sync режим працює без Kafka (тільки API service).

## Поточний стан

**Монолітна архітектура:**
```
cmd/app/main.go (один бінарник)
  ├─ HTTP server (webhooks + manual ops + reads)
  ├─ PostgreSQL + repositories + domain services
  ├─ Kafka publishers (якщо WEBHOOK_MODE=kafka)
  └─ Kafka consumers (якщо WEBHOOK_MODE=kafka)
```

**Проблема:** Неможливо масштабувати webhooks окремо від business logic. Один crash валить все.

## Архітектурні рішення

| Питання | Рішення | Чому |
|---------|---------|------|
| **Режими роботи** | Sync режим = тільки API service з WEBHOOK_MODE=sync (без Kafka). Kafka режим = API + Ingest | "Complex opt-in": простий режим працює без зайвих компонентів |
| **Залежності Ingest** | Gin HTTP server, Kafka publisher, Logger, Config (мінімум). **БЕЗ** domain services, repositories, PostgreSQL | Ingest - dumb edge service, не знає про domain |
| **Залежності API** | Повний стек: HTTP server, PostgreSQL, Domain services, Kafka consumers (якщо kafka режим) | API - brain з усією бізнес-логікою |
| **Webhook endpoints** | Sync режим: в API service. Kafka режим: в Ingest service | Гнучкість через конфіг |
| **Shared код** | Handlers з nullable dependencies, webhook.Processor interface | Мінімум дублювання, чистіші абстракції |
| **Міграції** | Тільки API service (він володіє БД) | Логічна відповідальність |
| **Backward compatibility** | cmd/app як deprecated alias для cmd/api | Поступова міграція production |

## Структура пакетів

### Нова структура cmd/:

```
cmd/
├── api/                     # Головний API service (renamed from app)
│   └── main.go
├── ingest/                  # Новий Ingest service
│   └── main.go
└── app/                     # DEPRECATED (видалимо пізніше)
    └── main.go
```

### Нові/модифіковані файли в internal/:

```
internal/
├── app/
│   ├── api.go              # API service bootstrap (рефакторинг app.go)
│   ├── workers.go          # Kafka consumers (без змін)
│   └── ...
├── ingest/                  # NEW
│   └── ingest.go           # Ingest service bootstrap
├── controller/rest/
│   ├── webhook_router.go   # NEW: Routes для Ingest (тільки webhooks)
│   ├── api_router.go       # NEW: Routes для API (manual ops + reads + webhooks в sync)
│   ├── router.go           # DEPRECATED (зберігаємо для compatibility)
│   └── handlers/
│       ├── order.go        # MODIFY: nullable service/processor
│       └── chargeback.go   # MODIFY: nullable service/processor
└── ...
```

### Що використовує кожен сервіс:

**Ingest Service:**
- `internal/ingest/ingest.go`
- `internal/controller/rest/webhook_router.go`
- `internal/controller/rest/handlers/{order,chargeback}.go`
- `internal/webhook/async.go`
- `internal/external/kafka/publisher.go`
- `pkg/logger/`

**API Service:**
- `internal/app/api.go` + `workers.go`
- `internal/controller/rest/api_router.go`
- `internal/controller/rest/handlers/` (всі)
- `internal/domain/`, `internal/repo/`
- `internal/external/{silvergate,opensearch,kafka/consumer.go}`
- `internal/webhook/sync.go` (тільки в sync режимі)

## Імплементація

### Фаза 1: Підготовка (без breaking changes)

**1.1. Створити config типи**

`config/config.go`:
```go
// IngestConfig - мінімальна конфігурація для Ingest service
type IngestConfig struct {
    Port      int      `env:"PORT" envDefault:"3001"`
    LogLevel  string   `env:"LOG_LEVEL" envDefault:"info"`

    // Kafka (required)
    KafkaBrokers         []string `env:"KAFKA_BROKERS" envSeparator:"," required:"true"`
    KafkaOrdersTopic     string   `env:"KAFKA_ORDERS_TOPIC" envDefault:"webhooks.orders"`
    KafkaDisputesTopic   string   `env:"KAFKA_DISPUTES_TOPIC" envDefault:"webhooks.disputes"`
}

// APIConfig - повна конфігурація для API service
type APIConfig struct {
    // Всі поля з поточного Config
    // + WebhookMode для вибору режиму
}

// Backward compatibility
type Config = APIConfig

func NewIngestConfig() (IngestConfig, error) { ... }
func NewAPIConfig() (APIConfig, error) { ... }
```

**Зміни:**
- Додати `IngestConfig` struct
- Додати `APIConfig` type (копія поточного `Config`)
- `Config` стає alias для `APIConfig`
- Функції `NewIngestConfig()`, `NewAPIConfig()`
- Зберегти `New()` як є для backward compatibility
- **ВАЖЛИВО:** OpenSearch не `required:"true"` в APIConfig (має бути опціональним)

**1.2. Створити роутери**

`internal/controller/rest/webhook_router.go`:
```go
type WebhookRouter struct {
    order      handlers.OrderHandler
    chargeback handlers.ChargebackHandler
}

func (r *WebhookRouter) SetUp(engine *gin.Engine) {
    engine.GET("/health", ...) // service: ingest
    engine.POST("/webhooks/payments/orders", r.order.Webhook)
    engine.POST("/webhooks/payments/chargebacks", r.chargeback.Webhook)
}
```

`internal/controller/rest/api_router.go`:
```go
type APIRouter struct {
    order           handlers.OrderHandler
    chargeback      handlers.ChargebackHandler
    dispute         handlers.DisputeHandler
    includeWebhooks bool  // true в sync режимі
}

func (r *APIRouter) SetUp(engine *gin.Engine) {
    engine.GET("/health", ...) // service: api

    // Webhooks тільки якщо sync режим
    if r.includeWebhooks {
        engine.POST("/webhooks/payments/orders", r.order.Webhook)
        engine.POST("/webhooks/payments/chargebacks", r.chargeback.Webhook)
    }

    // Manual operations + reads
    engine.GET("/orders", r.order.Filter)
    engine.GET("/orders/:order_id", r.order.Get)
    engine.POST("/orders/:order_id/hold", r.order.Hold)
    engine.POST("/orders/:order_id/capture", r.order.Capture)
    // ... disputes endpoints
}
```

**1.3. Модифікувати handlers**

`internal/controller/rest/handlers/order.go`:
```go
// OrderHandler тепер може мати nil service (Ingest mode) або nil processor (API kafka mode)
func NewOrderHandler(s *order.OrderService, processor webhook.Processor) OrderHandler {
    return OrderHandler{service: s, processor: processor}
}

func (h *OrderHandler) Webhook(c *gin.Context) {
    if h.processor == nil {
        c.JSON(503, gin.H{"message": "Webhook endpoint not available"})
        return
    }
    // Existing logic
}

func (h *OrderHandler) Get(c *gin.Context) {
    if h.service == nil {
        c.JSON(500, gin.H{"message": "Service not available"})
        return
    }
    // Existing logic
}
```

Аналогічно для `chargeback.go`.

**1.4. Тести**

- Unit тести для нових роутерів
- Unit тести для handlers з nil dependencies

### Фаза 2: Створення Ingest Service

**2.1. Ingest application layer**

`internal/ingest/ingest.go`:
```go
package ingest

import (
    "context"
    "fmt"
    "net/http"
    "os"
    "os/signal"
    "syscall"

    "TestTaskJustPay/config"
    "TestTaskJustPay/internal/app"
    "TestTaskJustPay/internal/controller/rest"
    "TestTaskJustPay/internal/controller/rest/handlers"
    "TestTaskJustPay/internal/external/kafka"
    "TestTaskJustPay/internal/webhook"
    "TestTaskJustPay/pkg/logger"
)

func Run(cfg config.IngestConfig) {
    l := logger.New(cfg.LogLevel)

    ctx, cancel := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
    defer cancel()

    engine := app.NewGinEngine(l)

    // Kafka publishers
    orderPublisher := kafka.NewPublisher(l, cfg.KafkaBrokers, cfg.KafkaOrdersTopic)
    disputePublisher := kafka.NewPublisher(l, cfg.KafkaBrokers, cfg.KafkaDisputesTopic)
    defer orderPublisher.Close()
    defer disputePublisher.Close()

    // AsyncProcessor
    processor := webhook.NewAsyncProcessor(orderPublisher, disputePublisher)

    // Handlers (service=nil для ingest)
    orderHandler := handlers.NewOrderHandler(nil, processor)
    chargebackHandler := handlers.NewChargebackHandler(nil, processor)

    // Webhook-only routes
    router := rest.NewWebhookRouter(orderHandler, chargebackHandler)
    router.SetUp(engine)

    // Start HTTP
    go func() {
        l.Info("Ingest service started: port=%d", cfg.Port)
        if err := engine.Run(fmt.Sprintf(":%d", cfg.Port)); err != nil && err != http.ErrServerClosed {
            l.Error("HTTP error: %v", err)
        }
    }()

    <-ctx.Done()
    l.Info("Shutting down Ingest service...")
}
```

**2.2. Entry point**

`cmd/ingest/main.go`:
```go
package main

import (
    "TestTaskJustPay/config"
    "TestTaskJustPay/internal/ingest"
    "log"
)

func main() {
    cfg, err := config.NewIngestConfig()
    if err != nil {
        log.Fatalf("Config error: %s", err)
    }
    ingest.Run(cfg)
}
```

**2.3. Інтеграційні тести**

`internal/ingest/ingest_integration_test.go`:
- Запустити Kafka в testcontainers
- Запустити Ingest service
- Надіслати webhook POST запит
- Перевірити що повідомлення з'явилося в Kafka topic

### Фаза 3: Рефакторинг API Service

**3.1. Рефакторинг app.go**

`internal/app/app.go` → модифікувати `Run()`:
```go
func Run(cfg config.APIConfig) {  // було Config
    l := logger.New(cfg.LogLevel)

    // ... existing initialization (pool, repos, services)

    // Webhook processor залежить від режиму
    var processor webhook.Processor
    if cfg.WebhookMode == "kafka" {
        l.Info("Webhook mode: kafka - webhooks handled by Ingest service, starting consumers")
        processor = nil  // API не обробляє webhooks по HTTP

        // Запустити Kafka consumers
        StartWorkers(ctx, l, cfg, orderService, disputeService)
    } else {
        l.Info("Webhook mode: sync - webhooks handled directly by API service")
        processor = webhook.NewSyncProcessor(orderService, disputeService)
    }

    // Handlers
    var orderHandler handlers.OrderHandler
    var chargebackHandler handlers.ChargebackHandler

    if cfg.WebhookMode == "sync" {
        // Sync: API обробляє webhooks
        orderHandler = handlers.NewOrderHandler(orderService, processor)
        chargebackHandler = handlers.NewChargebackHandler(disputeService, processor)
    } else {
        // Kafka: API НЕ обробляє webhooks
        orderHandler = handlers.NewOrderHandler(orderService, nil)
        chargebackHandler = handlers.NewChargebackHandler(disputeService, nil)
    }

    disputeHandler := handlers.NewDisputeHandler(disputeService)

    // API routes (webhooks тільки якщо sync)
    router := rest.NewAPIRouter(
        orderHandler,
        chargebackHandler,
        disputeHandler,
        cfg.WebhookMode == "sync",
    )
    router.SetUp(engine)

    // ... migrations, HTTP server start
}
```

**Примітка:** Можна залишити файл `app.go` як є, або rename в `api.go` для clarity.

**3.2. Entry point**

`cmd/api/main.go`:
```go
package main

import (
    "TestTaskJustPay/config"
    "TestTaskJustPay/internal/app"
    "log"
)

func main() {
    cfg, err := config.New()  // або NewAPIConfig()
    if err != nil {
        log.Fatalf("Config error: %s", err)
    }
    app.Run(cfg)
}
```

**3.3. Deprecated entry point**

`cmd/app/main.go`:
```go
package main

import (
    "TestTaskJustPay/config"
    "TestTaskJustPay/internal/app"
    "log"
)

func main() {
    log.Println("WARNING: cmd/app is deprecated, use cmd/api instead")
    cfg, err := config.New()
    if err != nil {
        log.Fatalf("Config error: %s", err)
    }
    app.Run(cfg)
}
```

### Фаза 4: Deployment та Development

**4.1. Makefile**

```makefile
# Default: sync режим (простий для dev)
run-dev: start_containers
	@echo "Running in SYNC mode (API only)"
	WEBHOOK_MODE=sync go run ./cmd/api

# Kafka режим: обидва сервіси (потребує goreman або два термінали)
run-kafka: start_containers
	@which goreman > /dev/null || (echo "Install: go install github.com/mattn/goreman@latest" && exit 1)
	@echo "Running in KAFKA mode (API + Ingest)"
	WEBHOOK_MODE=kafka goreman start

# Standalone targets
run-api: start_containers
	go run ./cmd/api

run-ingest:
	go run ./cmd/ingest

# Backward compatibility
run-sync: run-dev
```

**4.2. Procfile** (для goreman):

```
api: WEBHOOK_MODE=kafka go run ./cmd/api
ingest: go run ./cmd/ingest
```

**4.3. Environment files**

`.env.api.example`:
```bash
# API Service
PORT=3000
PG_URL=postgres://...
PG_POOL_MAX=2
LOG_LEVEL=info

# Silvergate
SILVERGATE_BASE_URL=http://localhost:8080
SILVERGATE_SUBMIT_REPRESENTMENT_PATH=/api/v1/dispute-representments/create
SILVERGATE_CAPTURE_PATH=/api/v1/capture

# OpenSearch (optional)
OPENSEARCH_URLS=http://opensearch-node1:9200
OPENSEARCH_INDEX_DISPUTES=events-disputes
OPENSEARCH_INDEX_ORDERS=events-orders

# Webhook mode
WEBHOOK_MODE=sync  # "sync" or "kafka"

# Kafka (required only if WEBHOOK_MODE=kafka)
KAFKA_BROKERS=localhost:9092
KAFKA_ORDERS_TOPIC=webhooks.orders
KAFKA_DISPUTES_TOPIC=webhooks.disputes
KAFKA_ORDERS_CONSUMER_GROUP=payment-app-orders
KAFKA_DISPUTES_CONSUMER_GROUP=payment-app-disputes
KAFKA_ORDERS_DLQ_TOPIC=webhooks.orders.dlq
KAFKA_DISPUTES_DLQ_TOPIC=webhooks.disputes.dlq
```

`.env.ingest.example`:
```bash
# Ingest Service
PORT=3001
LOG_LEVEL=info

# Kafka (required)
KAFKA_BROKERS=localhost:9092
KAFKA_ORDERS_TOPIC=webhooks.orders
KAFKA_DISPUTES_TOPIC=webhooks.disputes
```

**4.4. Dockerfiles**

`Dockerfile.api`:
```dockerfile
FROM golang:1.23-alpine AS builder
WORKDIR /app
COPY go.mod go.sum ./
RUN go mod download
COPY . .
RUN CGO_ENABLED=0 GOOS=linux go build -o /api ./cmd/api

FROM alpine:latest
RUN apk --no-cache add ca-certificates
COPY --from=builder /api /api
ENTRYPOINT ["/api"]
```

`Dockerfile.ingest`:
```dockerfile
FROM golang:1.23-alpine AS builder
WORKDIR /app
COPY go.mod go.sum ./
RUN go mod download
COPY . .
RUN CGO_ENABLED=0 GOOS=linux go build -o /ingest ./cmd/ingest

FROM alpine:latest
RUN apk --no-cache add ca-certificates
COPY --from=builder /ingest /ingest
ENTRYPOINT ["/ingest"]
```

**4.5. docker-compose.yaml**

```yaml
services:
  api-service:
    image: payment-api
    container_name: payment-api
    profiles: [prod]
    build:
      context: .
      dockerfile: Dockerfile.api
    depends_on:
      - db
      - opensearch-node1
      - kafka-init
    env_file: .env.api
    environment:
      - PG_URL=user=postgres password=secret host=db port=5432 dbname=payments sslmode=disable
      - OPENSEARCH_URLS=http://opensearch-node1:9200
      - KAFKA_BROKERS=kafka:29092
      - WEBHOOK_MODE=kafka
    ports:
      - "3000:3000"

  ingest-service:
    image: payment-ingest
    container_name: payment-ingest
    profiles: [prod]
    build:
      context: .
      dockerfile: Dockerfile.ingest
    depends_on:
      - kafka-init
    env_file: .env.ingest
    environment:
      - KAFKA_BROKERS=kafka:29092
    ports:
      - "3001:3001"

  # ... rest (db, kafka, opensearch)
```

### Фаза 5: Документація

**5.1. README.md оновлення**

Додати секцію:

```markdown
## Services Architecture

The system consists of two services:

### API Service (cmd/api)
- Manual operations: capture, hold
- Read endpoints: GET /orders, /disputes
- Webhook processing (sync mode only)
- Kafka consumers (kafka mode)
- Database migrations

### Ingest Service (cmd/ingest)
- Lightweight HTTP → Kafka gateway
- Accepts webhooks from payment providers
- Publishes to Kafka topics
- No database, no business logic

## Running Modes

**Sync mode (default for dev):**
```bash
make run-dev  # Runs API service only with WEBHOOK_MODE=sync
```

**Kafka mode (production):**
```bash
make run-kafka  # Runs both API + Ingest services
```
```

**5.2. CLAUDE.md оновлення**

Змінити секцію "Project Overview" та "Commands".

## Файли для модифікації

| Файл | Операція | Опис |
|------|----------|------|
| **Нові файли** | | |
| `config/config.go` | MODIFY | Додати `IngestConfig`, `APIConfig` |
| `internal/controller/rest/webhook_router.go` | CREATE | Роутер для Ingest |
| `internal/controller/rest/api_router.go` | CREATE | Роутер для API |
| `internal/ingest/ingest.go` | CREATE | Bootstrap Ingest service |
| `cmd/ingest/main.go` | CREATE | Entry point Ingest |
| `cmd/api/main.go` | CREATE | Entry point API |
| `Procfile` | CREATE | Goreman config |
| `.env.api.example` | CREATE | API config template |
| `.env.ingest.example` | CREATE | Ingest config template |
| `Dockerfile.api` | CREATE | Docker image API |
| `Dockerfile.ingest` | CREATE | Docker image Ingest |
| **Модифіковані** | | |
| `internal/app/app.go` | MODIFY | Підтримка kafka режиму без webhook endpoints |
| `internal/controller/rest/handlers/order.go` | MODIFY | Nullable service/processor |
| `internal/controller/rest/handlers/chargeback.go` | MODIFY | Nullable service/processor |
| `cmd/app/main.go` | MODIFY | Deprecated warning |
| `Makefile` | MODIFY | Нові targets: run-kafka, run-api, run-ingest |
| `docker-compose.yaml` | MODIFY | Додати api-service, ingest-service |
| `README.md` | MODIFY | Документація архітектури |
| `CLAUDE.md` | MODIFY | Оновити commands та overview |

## Порядок імплементації

1. **Config types** → Фундамент для розділення
2. **Routers** → Визначити інтерфейси сервісів
3. **Handlers modifications** → Підтримка nullable dependencies
4. **Ingest service** → Легкий gateway (можна тестувати окремо)
5. **API service refactoring** → Основна логіка
6. **Deployment** → Makefile, Docker, env files
7. **Documentation** → README, CLAUDE.md
8. **Testing** → Integration tests для обох сервісів

## Trade-offs

**Обрані рішення:**
- ✅ Sync режим тільки в API (простіше для dev)
- ✅ Shared handlers з nullable deps (мінімум дублювання)
- ✅ Міграції тільки в API (логічна відповідальність)
- ✅ Flat internal/ structure (простіше шерити код)

**Альтернативи (відхилені):**
- ❌ Два окремі монорепо (складніше management)
- ❌ gRPC між сервісами (over-engineering для першого кроку)
- ❌ Sync режим в Ingest (порушує ідею легкого gateway)

## Критичні файли

- `config/config.go` - типи конфігурації
- `internal/app/app.go` - рефакторинг API bootstrap
- `internal/ingest/ingest.go` - Ingest bootstrap
- `internal/controller/rest/handlers/order.go` - nullable dependencies
- `Makefile` - developer experience
