# План: Subtask 3 - Health Checks

## Мета

Додати Kubernetes-style health check endpoints для обох сервісів: liveness probe (процес живий) та readiness probe (залежності доступні).

## Endpoints

| Endpoint | Призначення | Що перевіряє | HTTP Status |
|----------|-------------|--------------|-------------|
| `/health/live` | Liveness probe | Процес відповідає | 200 OK |
| `/health/ready` | Readiness probe | Dependencies OK | 200 OK / 503 Service Unavailable |

## Залежності для перевірки

**API Service:**
- PostgreSQL (завжди)
- Kafka brokers (тільки якщо `webhook_mode=kafka`)

**Ingest Service:**
- Kafka brokers (тільки якщо `webhook_mode=kafka`)
- В `webhook_mode=http` — немає зовнішніх залежностей, readiness завжди up

## Архітектурні рішення

| Питання | Рішення | Чому |
|---------|---------|------|
| Розміщення коду | `pkg/health/` | Shared між сервісами |
| Interface pattern | `Checker` interface | Легко додавати нові checks |
| Execution | Parallel | Швидше при багатьох checks |
| Timeout | Context з 5s timeout | Не блокувати probe надовго |
| Aggregation | All must pass | Консервативний підхід для readiness |
| Backward compat | Видалити старий `/health` | Не потрібна |

## Структура пакету

```
pkg/health/
├── checker.go    # Checker interface + Result type
├── postgres.go   # PostgreSQL checker (uses Pool.Ping)
├── kafka.go      # Kafka broker checker (uses Dial)
├── registry.go   # Registry to collect multiple checkers
└── handler.go    # Gin handlers for /health/live and /health/ready
```

## Імплементація

### 1. pkg/health/checker.go

```go
package health

import (
	"context"
	"time"
)

// DefaultTimeout is the default timeout for health checks.
const DefaultTimeout = 5 * time.Second

// Status represents the health status of a component.
type Status string

const (
	StatusUp   Status = "up"
	StatusDown Status = "down"
)

// Result is the outcome of a single health check.
type Result struct {
	Status  Status `json:"status"`
	Message string `json:"message,omitempty"`
}

// Checker is the interface for health check implementations.
type Checker interface {
	// Name returns the name of the component being checked.
	Name() string
	// Check performs the health check and returns the result.
	Check(ctx context.Context) Result
}
```

### 2. pkg/health/postgres.go

```go
package health

import (
	"context"

	"github.com/jackc/pgx/v5/pgxpool"
)

// PostgresChecker checks PostgreSQL connectivity.
type PostgresChecker struct {
	pool *pgxpool.Pool
}

// NewPostgresChecker creates a new PostgreSQL health checker.
func NewPostgresChecker(pool *pgxpool.Pool) *PostgresChecker {
	return &PostgresChecker{pool: pool}
}

// Name returns "postgres".
func (c *PostgresChecker) Name() string {
	return "postgres"
}

// Check pings the PostgreSQL database.
func (c *PostgresChecker) Check(ctx context.Context) Result {
	if err := c.pool.Ping(ctx); err != nil {
		return Result{Status: StatusDown, Message: err.Error()}
	}
	return Result{Status: StatusUp}
}
```

### 3. pkg/health/kafka.go

```go
package health

import (
	"context"

	"github.com/segmentio/kafka-go"
)

// KafkaChecker checks Kafka broker connectivity.
type KafkaChecker struct {
	brokers []string
}

// NewKafkaChecker creates a new Kafka health checker.
func NewKafkaChecker(brokers []string) *KafkaChecker {
	return &KafkaChecker{brokers: brokers}
}

// Name returns "kafka".
func (c *KafkaChecker) Name() string {
	return "kafka"
}

// Check attempts to connect to any Kafka broker.
func (c *KafkaChecker) Check(ctx context.Context) Result {
	for _, broker := range c.brokers {
		conn, err := kafka.DialContext(ctx, "tcp", broker)
		if err == nil {
			conn.Close()
			return Result{Status: StatusUp}
		}
	}
	return Result{Status: StatusDown, Message: "all brokers unreachable"}
}
```

### 4. pkg/health/registry.go

```go
package health

import (
	"context"
	"sync"
)

// Registry holds multiple health checkers.
type Registry struct {
	checkers []Checker
}

// NewRegistry creates a new health check registry.
func NewRegistry(checkers ...Checker) *Registry {
	return &Registry{checkers: checkers}
}

// CheckResult is the result of a single named check.
type CheckResult struct {
	Name    string `json:"name"`
	Status  Status `json:"status"`
	Message string `json:"message,omitempty"`
}

// ReadinessResponse is the aggregated readiness check response.
type ReadinessResponse struct {
	Status Status        `json:"status"`
	Checks []CheckResult `json:"checks,omitempty"`
}

// CheckAll runs all registered checkers in parallel.
func (r *Registry) CheckAll(ctx context.Context) ReadinessResponse {
	if len(r.checkers) == 0 {
		return ReadinessResponse{Status: StatusUp}
	}

	results := make([]CheckResult, len(r.checkers))
	var wg sync.WaitGroup

	for i, checker := range r.checkers {
		wg.Add(1)
		go func(idx int, c Checker) {
			defer wg.Done()
			res := c.Check(ctx)
			results[idx] = CheckResult{
				Name:    c.Name(),
				Status:  res.Status,
				Message: res.Message,
			}
		}(i, checker)
	}

	wg.Wait()

	overall := StatusUp
	for _, res := range results {
		if res.Status == StatusDown {
			overall = StatusDown
			break
		}
	}

	return ReadinessResponse{Status: overall, Checks: results}
}
```

### 5. pkg/health/handler.go

```go
package health

import (
	"context"
	"net/http"
	"time"

	"github.com/gin-gonic/gin"
)

// LivenessHandler returns a handler for liveness probes.
// Always returns 200 OK if the process is running.
func LivenessHandler() gin.HandlerFunc {
	return func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{"status": StatusUp})
	}
}

// ReadinessHandler returns a handler for readiness probes.
// Returns 200 OK if all checks pass, 503 Service Unavailable otherwise.
func ReadinessHandler(registry *Registry, timeout time.Duration) gin.HandlerFunc {
	return func(c *gin.Context) {
		ctx, cancel := context.WithTimeout(c.Request.Context(), timeout)
		defer cancel()

		response := registry.CheckAll(ctx)

		status := http.StatusOK
		if response.Status == StatusDown {
			status = http.StatusServiceUnavailable
		}

		c.JSON(status, response)
	}
}
```

## Файли для модифікації

| Файл | Зміни |
|------|-------|
| `pkg/health/checker.go` | **NEW** - Interface та types |
| `pkg/health/postgres.go` | **NEW** - PostgreSQL checker |
| `pkg/health/kafka.go` | **NEW** - Kafka checker |
| `pkg/health/registry.go` | **NEW** - Registry для checkers |
| `pkg/health/handler.go` | **NEW** - Gin handlers |
| `internal/api/app.go` | Створити registry, передати в router |
| `internal/api/router.go` | Замінити `/health` на `/health/live` та `/health/ready` |
| `internal/ingest/app.go` | Створити registry, передати в router |
| `internal/ingest/router.go` | Замінити `/health` на `/health/live` та `/health/ready` |

## Зміни в API Service

### internal/api/app.go

```go
// After pool creation, create health registry
var healthCheckers []health.Checker
healthCheckers = append(healthCheckers, health.NewPostgresChecker(pool.Pool))

if cfg.WebhookMode == "kafka" {
    healthCheckers = append(healthCheckers, health.NewKafkaChecker(cfg.KafkaBrokers))
}

healthRegistry := health.NewRegistry(healthCheckers...)

// Pass to router
router := NewRouter(orderHandler, chargebackHandler, disputeHandler, healthRegistry)
```

### internal/api/router.go

```go
type Router struct {
    order          *handlers.OrderHandler
    chargeback     *handlers.ChargebackHandler
    dispute        *handlers.DisputeHandler
    healthRegistry *health.Registry
}

func (r *Router) SetUp(engine *gin.Engine) {
    // Health checks
    engine.GET("/health/live", health.LivenessHandler())
    engine.GET("/health/ready", health.ReadinessHandler(r.healthRegistry, health.DefaultTimeout))

    // ... rest of routes
}
```

## Зміни в Ingest Service

### internal/ingest/app.go

```go
// Create health registry based on mode
var healthCheckers []health.Checker

if cfg.WebhookMode == "kafka" {
    healthCheckers = append(healthCheckers, health.NewKafkaChecker(cfg.KafkaBrokers))
}
// HTTP mode: no external dependencies to check

healthRegistry := health.NewRegistry(healthCheckers...)

// Pass to router
router := NewRouter(orderHandler, chargebackHandler, healthRegistry)
```

### internal/ingest/router.go

```go
type Router struct {
    order          *handlers.OrderHandler
    chargeback     *handlers.ChargebackHandler
    healthRegistry *health.Registry
}

func (r *Router) SetUp(engine *gin.Engine) {
    // Health checks
    engine.GET("/health/live", health.LivenessHandler())
    engine.GET("/health/ready", health.ReadinessHandler(r.healthRegistry, health.DefaultTimeout))

    // ... rest of routes
}
```

## Порядок імплементації

1. Створити `pkg/health/checker.go`
2. Створити `pkg/health/postgres.go`
3. Створити `pkg/health/kafka.go`
4. Створити `pkg/health/registry.go`
5. Створити `pkg/health/handler.go`
6. Оновити `internal/api/app.go` - створити registry
7. Оновити `internal/api/router.go` - додати endpoints
8. Оновити `internal/ingest/app.go` - створити registry
9. Оновити `internal/ingest/router.go` - додати endpoints

## Verification

```bash
# Start in HTTP mode (default)
make run-dev

# Check liveness (API)
curl http://localhost:3000/health/live
# Expected: {"status":"up"}

# Check readiness (API) - only Postgres
curl http://localhost:3000/health/ready
# Expected: {"status":"up","checks":[{"name":"postgres","status":"up"}]}

# Start in Kafka mode
make run-kafka

# Check readiness (API) - Postgres + Kafka
curl http://localhost:3000/health/ready
# Expected: {"status":"up","checks":[{"name":"postgres","status":"up"},{"name":"kafka","status":"up"}]}

# Check readiness (Ingest) - only Kafka
curl http://localhost:3001/health/ready
# Expected: {"status":"up","checks":[{"name":"kafka","status":"up"}]}

# Test failure scenario - stop Postgres
docker stop dpm-postgres
curl http://localhost:3000/health/ready
# Expected: HTTP 503, {"status":"down","checks":[{"name":"postgres","status":"down","message":"..."}]}
```

## Response Examples

**Liveness (always):**
```json
{"status":"up"}
```

**Readiness (all healthy):**
```json
{
  "status": "up",
  "checks": [
    {"name": "postgres", "status": "up"},
    {"name": "kafka", "status": "up"}
  ]
}
```

**Readiness (degraded):**
```json
{
  "status": "down",
  "checks": [
    {"name": "postgres", "status": "up"},
    {"name": "kafka", "status": "down", "message": "all brokers unreachable"}
  ]
}
```

**Readiness (no dependencies - Ingest HTTP mode):**
```json
{"status":"up"}
```
