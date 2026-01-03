# План: Monorepo Architecture Refactoring

**Feature:** 002-ingest-service-extraction
**Subtask:** 1.5 (новий, між Subtask 1 та Subtask 2)

## Мета

Реорганізувати `internal/` з layer-based на service-based структуру для монорепо з мікросервісами. Підготувати до потенційного виділення сервісів у окремі репозиторії.

## Проблеми поточної архітектури

1. **Nullable dependencies в handlers** - `if h.processor == nil` костиль
2. **Змінна `includeWebhooks`** - implicit логіка
3. **Один набір handlers для всіх сервісів** - не масштабується
4. **Layer-based structure** - не підходить для мікросервісів

## Цільова структура

```
internal/
├── api/                      # API Service (може стати окремим репо)
│   ├── handlers/             # HTTP handlers
│   │   ├── order.go          # GET, POST /orders/*
│   │   ├── dispute.go        # GET, POST /disputes/*
│   │   └── chargeback.go     # якщо потрібен (або merge з dispute)
│   ├── consumers/            # Kafka consumer handlers
│   │   ├── order.go          # <- from internal/controller/message/order.go
│   │   └── dispute.go        # <- from internal/controller/message/dispute.go
│   ├── router.go             # API routes (без webhook endpoints)
│   └── service.go            # Bootstrap (current app.go)
│
├── ingest/                   # Ingest Service (може стати окремим репо)
│   ├── handlers/
│   │   ├── order.go          # POST /webhooks/payments/orders
│   │   └── chargeback.go     # POST /webhooks/payments/chargebacks
│   ├── router.go             # Webhook routes only
│   └── service.go            # Bootstrap (lightweight)
│
├── shared/                   # Shared code (може стати спільним модулем)
│   ├── domain/               # <- move from internal/domain/
│   │   ├── order/
│   │   ├── dispute/
│   │   └── gateway/
│   ├── repo/                 # <- move from internal/repo/
│   │   ├── order/
│   │   ├── dispute/
│   │   ├── dispute_eventsink/
│   │   └── order_eventsink/
│   ├── external/             # <- move from internal/external/
│   │   ├── kafka/
│   │   ├── silvergate/
│   │   └── opensearch/
│   ├── webhook/              # <- move from internal/webhook/
│   └── testinfra/            # shared test infrastructure
│
├── migrations/               # DB migrations (API owns DB)
│   └── *.sql
│
└── messaging/                # <- move from internal/messaging/ (if exists)
```

## Принципи

1. **Service isolation**: api/ та ingest/ НЕ імпортують один одного
2. **Shared via shared/**: вся спільна логіка тільки через shared/
3. **No nullable deps**: кожен handler має чіткі required dependencies
4. **Clear boundaries**: підготовка до gRPC/Kafka-only комунікації

## Порядок імплементації

### Фаза 1: Створити структуру папок
- [ ] Створити `internal/shared/`
- [ ] Створити `internal/api/handlers/`
- [ ] Створити `internal/api/consumers/`
- [ ] Створити `internal/ingest/handlers/`
- [ ] Створити `internal/migrations/`

### Фаза 2: Перемістити shared код
- [ ] `internal/domain/` → `internal/shared/domain/`
- [ ] `internal/repo/` → `internal/shared/repo/`
- [ ] `internal/external/` → `internal/shared/external/`
- [ ] `internal/webhook/` → `internal/shared/webhook/`
- [ ] `internal/app/migrations/` → `internal/migrations/`
- [ ] Оновити всі import paths

### Фаза 3: Розділити handlers
- [ ] Створити `internal/api/handlers/order.go` (без processor, тільки service)
- [ ] Створити `internal/api/handlers/dispute.go`
- [ ] Створити `internal/api/handlers/chargeback.go` (або merge)
- [ ] Перемістити `internal/controller/message/` → `internal/api/consumers/`
- [ ] Створити `internal/ingest/handlers/order.go` (тільки processor)
- [ ] Створити `internal/ingest/handlers/chargeback.go`
- [ ] Видалити `internal/controller/` (старі handlers)

### Фаза 4: Оновити routers та bootstrap
- [ ] `internal/api/router.go` - без includeWebhooks, чіткі routes
- [ ] `internal/api/service.go` - рефакторинг з app.go
- [ ] `internal/ingest/router.go` - тільки webhook routes
- [ ] Rename `internal/ingest/ingest.go` → `internal/ingest/service.go`

### Фаза 5: Cleanup
- [ ] Видалити `internal/controller/rest/` (старі routers)
- [ ] Видалити `internal/app/` (перенесено в api/)
- [ ] Видалити nullable dependency checks з handlers
- [ ] Оновити cmd/api та cmd/ingest imports

### Фаза 6: Тести та документація
- [ ] Перевірити що все компілюється
- [ ] Unit tests проходять
- [ ] Оновити CLAUDE.md з новою структурою
- [ ] Оновити feature README

## Файли для зміни

### Нові файли
- `internal/api/handlers/order.go`
- `internal/api/handlers/dispute.go`
- `internal/api/consumers/order.go`
- `internal/api/consumers/dispute.go`
- `internal/api/router.go`
- `internal/api/service.go`
- `internal/api/workers.go` (Kafka consumer management)
- `internal/api/gin_engine.go`
- `internal/api/migration.go`
- `internal/ingest/handlers/order.go`
- `internal/ingest/handlers/chargeback.go`
- `internal/ingest/router.go`
- `internal/ingest/service.go`

### Файли для переміщення (зі зміною imports)
- `internal/domain/**` → `internal/shared/domain/**`
- `internal/repo/**` → `internal/shared/repo/**`
- `internal/external/**` → `internal/shared/external/**`
- `internal/webhook/**` → `internal/shared/webhook/**`
- `internal/app/migrations/**` → `internal/migrations/**`
- `internal/messaging/**` → `internal/shared/messaging/**`

### Файли для видалення (після переміщення)
- `internal/controller/` (вся папка)
- `internal/app/` (вся папка)
- `internal/domain/` (перенесено в shared)
- `internal/repo/` (перенесено в shared)
- `internal/external/` (перенесено в shared)
- `internal/webhook/` (перенесено в shared)
- `internal/messaging/` (перенесено в shared)

## Handler Design

### API Handler (чистий, без processor)
```go
// internal/api/handlers/order.go
type OrderHandler struct {
    service *order.OrderService  // required, never nil
}

func NewOrderHandler(s *order.OrderService) *OrderHandler {
    return &OrderHandler{service: s}
}

func (h *OrderHandler) Get(c *gin.Context) { ... }
func (h *OrderHandler) Filter(c *gin.Context) { ... }
func (h *OrderHandler) Hold(c *gin.Context) { ... }
func (h *OrderHandler) Capture(c *gin.Context) { ... }
// НЕ має Webhook() метод
```

### Ingest Handler (чистий, без service)
```go
// internal/ingest/handlers/order.go
type OrderHandler struct {
    processor webhook.Processor  // required, never nil
}

func NewOrderHandler(p webhook.Processor) *OrderHandler {
    return &OrderHandler{processor: p}
}

func (h *OrderHandler) Webhook(c *gin.Context) { ... }
// НЕ має Get/Filter/Hold/Capture
```

## Ризики

1. **Багато змін import paths** - може зламати щось несподівано
2. **Git history** - переміщення файлів ускладнює blame
3. **IDE refactoring** - може не все підхопити автоматично

## Мітигація

- Робити покроково з перевіркою на кожному етапі
- `make test` після кожної фази
- Commit після кожної фази для rollback
