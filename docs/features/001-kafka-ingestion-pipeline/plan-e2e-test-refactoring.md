# Plan: E2E Test Refactoring (Docker-based)

## Мета

Переробити інтеграційні тести з in-process `setupTestServer()` на повний Docker-based E2E:
- Всі сервіси (API, Ingest) запускаються як Docker контейнери через testcontainers
- Тестується реальний deployment flow: Dockerfile → container → health check → traffic
- Підтримка обох deployment modes: Kafka (async) та HTTP (sync)
- Нуль дублювання коду з `app.go`

## Поточний стан

`setupTestServer()` в `integration-test/integration_test.go` (~75 рядків) дублює bootstrap з `internal/api/app.go`:
- Repository, service, handler constructors
- Kafka publisher/consumer setup
- Router setup з обома API і webhook endpoints в одному Gin engine

Ключові проблеми:
1. **Дублювання** — при зміні `app.go` треба оновлювати тести
2. **Не реалістична архітектура** — API і Ingest в одному процесі/сервері
3. **Тестує лише Kafka mode** — HTTP sync mode не тестується
4. **Не тестує Dockerfile** — проблеми з Docker image зловляться лише на деплої

## Архітектура

```
┌────────────────┐     ┌───────────────────────┐     ┌──────────────────────┐
│   Test Code    │────▶│  Ingest Container      │────▶│   API Container      │
│   (go test)    │     │  (Dockerfile.ingest)   │     │   (Dockerfile.api)   │
└────────────────┘     └───────────────────────┘     └──────────────────────┘
       │                        │                            │
       │                        ▼                            ▼
       │                 ┌─────────────┐             ┌─────────────┐
       │                 │   Kafka     │             │  PostgreSQL  │
       │                 │ (container) │             │  (container) │
       │                 └─────────────┘             └─────────────┘
       │                                                     │
       │                 ┌─────────────┐                     │
       └────────────────▶│  Wiremock   │                     │
                         │ (container) │                     │
                         └─────────────┘                     │
       │                                                     │
       └─────── direct DB access (truncate, fixtures) ───────┘

Networking:
  - All containers on shared Docker network ("e2e-net")
  - Inter-container: DNS aliases (postgres, kafka, wiremock, api, ingest)
  - Test → containers: host-mapped ports
```

### Kafka mode flow
```
Test → POST webhook → Ingest(:3001) → Kafka(kafka:29092) → API consumer → DB
Test → GET /orders  → API(:3000) → PostgreSQL(postgres:5432) → response
```

### HTTP mode flow
```
Test → POST webhook → Ingest(:3001) → HTTP POST → API(:3000/internal/updates) → DB
Test → GET /orders  → API(:3000) → PostgreSQL(postgres:5432) → response
```

## Архітектурні рішення

| Питання | Рішення | Чому |
|---------|---------|------|
| Як запускати сервіси? | Docker containers via testcontainers | Тестує реальні Dockerfiles, нуль дублювання, консистентно з infra containers |
| Docker networking | Shared `testcontainers.Network`, DNS aliases | Контейнери знаходять один одного по hostname (postgres, kafka, api) |
| Image build | `testcontainers.FromDockerfile` з project root context | Використовує існуючі Dockerfile.api / Dockerfile.ingest |
| Коли білдити images? | Один раз в `TestMain` | Повторний build при кожному тесті занадто повільний |
| Коли стартувати контейнери? | Infra + сервіси в `TestMain`, truncate per test | Перезапуск контейнерів (~30s) vs truncate (~10ms) |
| Backward compatibility testinfra | Options pattern (`WithNetwork`) | Repo-level integration тести працюють без змін |
| Fixture loading | Direct DB access через `suite.Postgres.Pool` | Тест код має доступ до Postgres pool через host-mapped port |
| Health check | Poll `/health/ready` endpoint | Реалістично — як Kubernetes readiness probe |

## Структура пакетів

```
internal/shared/testinfra/
├── suite.go                ← MODIFY: add WithE2E option, network management
├── postgres.go             ← MODIFY: accept optional network config
├── kafka.go                ← MODIFY: accept optional network config
├── wiremock.go             ← MODIFY: accept optional network config
├── network.go              ← NEW: Docker network creation helper
├── api_container.go        ← NEW: API service container
└── ingest_container.go     ← NEW: Ingest service container

integration-test/
├── main_test.go            ← NEW: TestMain with Docker setup
├── helpers_test.go         ← NEW: E2EClient, HTTP helpers, wait functions
├── e2e_kafka_test.go       ← NEW: Kafka mode E2E tests
├── e2e_http_test.go        ← NEW: HTTP mode E2E tests
└── integration_test.go     ← DELETE: replaced by above files
```

## Імплементація

### Step 1: Network support in testinfra

Add optional Docker network to container constructors.

```go
// network.go
package testinfra

type NetworkConfig struct {
    Network      *testcontainers.DockerNetwork
    NetworkName  string
    Aliases      []string
}
```

Update `SuiteOptions`:
```go
type SuiteOptions struct {
    WithKafka    bool
    WithWiremock bool
    MappingsPath string
    // NEW
    WithE2E      bool   // enables Docker network + service containers
    ProjectRoot  string // path to project root (for Dockerfile builds)
}
```

Update container constructors to accept `*NetworkConfig`:
```go
func NewPostgres(ctx context.Context, netCfg *NetworkConfig) (*PostgresContainer, error) {
    req := testcontainers.ContainerRequest{
        Image: "pg17-partman:local",
        // ... existing config ...
    }

    if netCfg != nil {
        req.Networks = []string{netCfg.NetworkName}
        req.NetworkAliases = map[string][]string{
            netCfg.NetworkName: netCfg.Aliases,
        }
    }
    // ...
}
```

Same pattern for `NewKafka()` and `NewWiremock()`.

**Backward compatibility:** when `netCfg == nil` (repo-level tests), behavior is identical to current.

### Step 2: API container

```go
// api_container.go
package testinfra

type APIContainer struct {
    Container testcontainers.Container
    BaseURL   string // host-mapped URL for test code
}

type APIContainerConfig struct {
    PgDSN              string // internal Docker DNS, e.g. "postgres://postgres:secret@postgres:5432/payments_test"
    KafkaBrokers       string // internal, e.g. "kafka:29092"
    KafkaTopics        KafkaTopicNames
    SilvergateBaseURL  string // internal, e.g. "http://wiremock:8080"
    WebhookMode        string // "kafka" or "sync"
    ProjectRoot        string // for Dockerfile context
    Network            *NetworkConfig
}

func NewAPIContainer(ctx context.Context, cfg APIContainerConfig) (*APIContainer, error) {
    env := map[string]string{
        "PORT":           "3000",
        "PG_URL":         cfg.PgDSN,
        "PG_POOL_MAX":    "5",
        "WEBHOOK_MODE":   cfg.WebhookMode,
        "SILVERGATE_BASE_URL":                  cfg.SilvergateBaseURL,
        "SILVERGATE_SUBMIT_REPRESENTMENT_PATH": "/api/v1/dispute-representments/create",
        "SILVERGATE_CAPTURE_PATH":              "/api/v1/capture",
        "LOG_LEVEL":      "info",
        "LOG_FORMAT":     "console",
    }

    if cfg.WebhookMode == "kafka" {
        env["KAFKA_BROKERS"] = cfg.KafkaBrokers
        env["KAFKA_ORDERS_TOPIC"] = cfg.KafkaTopics.Orders
        env["KAFKA_DISPUTES_TOPIC"] = cfg.KafkaTopics.Disputes
        env["KAFKA_ORDERS_CONSUMER_GROUP"] = cfg.KafkaTopics.OrdersGroup
        env["KAFKA_DISPUTES_CONSUMER_GROUP"] = cfg.KafkaTopics.DisputesGroup
        env["KAFKA_ORDERS_DLQ_TOPIC"] = cfg.KafkaTopics.Orders + ".dlq"
        env["KAFKA_DISPUTES_DLQ_TOPIC"] = cfg.KafkaTopics.Disputes + ".dlq"
    }

    req := testcontainers.ContainerRequest{
        FromDockerfile: testcontainers.FromDockerfile{
            Context:    cfg.ProjectRoot,
            Dockerfile: "Dockerfile.api",
        },
        ExposedPorts: []string{"3000/tcp"},
        Env:          env,
        Networks:     []string{cfg.Network.NetworkName},
        NetworkAliases: map[string][]string{
            cfg.Network.NetworkName: {"api"},
        },
        WaitingFor: wait.ForHTTP("/health/ready").
            WithPort("3000/tcp").
            WithStartupTimeout(120 * time.Second),
    }

    container, err := testcontainers.GenericContainer(ctx, testcontainers.GenericContainerRequest{
        ContainerRequest: req,
        Started:          true,
    })
    // ... get host:port, return APIContainer ...
}
```

### Step 3: Ingest container

```go
// ingest_container.go
package testinfra

type IngestContainer struct {
    Container testcontainers.Container
    BaseURL   string
}

type IngestContainerConfig struct {
    WebhookMode  string // "kafka" or "http"
    KafkaBrokers string
    KafkaTopics  KafkaTopicNames
    APIBaseURL   string // for HTTP mode: "http://api:3000"
    ProjectRoot  string
    Network      *NetworkConfig
}

func NewIngestContainer(ctx context.Context, cfg IngestContainerConfig) (*IngestContainer, error) {
    env := map[string]string{
        "PORT":         "3001",
        "WEBHOOK_MODE": cfg.WebhookMode,
        "LOG_LEVEL":    "info",
        "LOG_FORMAT":   "console",
    }

    switch cfg.WebhookMode {
    case "kafka":
        env["KAFKA_BROKERS"] = cfg.KafkaBrokers
        env["KAFKA_ORDERS_TOPIC"] = cfg.KafkaTopics.Orders
        env["KAFKA_DISPUTES_TOPIC"] = cfg.KafkaTopics.Disputes
    case "http":
        env["API_BASE_URL"] = cfg.APIBaseURL
    }

    req := testcontainers.ContainerRequest{
        FromDockerfile: testcontainers.FromDockerfile{
            Context:    cfg.ProjectRoot,
            Dockerfile: "Dockerfile.ingest",
        },
        ExposedPorts: []string{"3001/tcp"},
        Env:          env,
        Networks:     []string{cfg.Network.NetworkName},
        NetworkAliases: map[string][]string{
            cfg.Network.NetworkName: {"ingest"},
        },
        WaitingFor: wait.ForHTTP("/health/ready").
            WithPort("3001/tcp").
            WithStartupTimeout(90 * time.Second),
    }
    // ...
}
```

### Step 4: Update TestSuite

```go
// suite.go additions

type TestSuite struct {
    Postgres  *PostgresContainer
    Kafka     *KafkaContainer
    Wiremock  *WiremockContainer
    // NEW
    API       *APIContainer
    Ingest    *IngestContainer
    Network   *testcontainers.DockerNetwork
}

func NewTestSuite(ctx context.Context, opts SuiteOptions) (*TestSuite, error) {
    suite := &TestSuite{}

    // Create network if E2E mode
    var netCfg *NetworkConfig
    if opts.WithE2E {
        network, err := CreateNetwork(ctx, "e2e-net")
        // ...
        suite.Network = network
        netCfg = &NetworkConfig{Network: network, NetworkName: "e2e-net"}
    }

    // Start infra containers in parallel (pass netCfg)
    // ... existing parallel start logic, but pass netCfg to constructors ...

    // Start service containers (after infra is ready)
    if opts.WithE2E {
        // Build internal DSN (via Docker network aliases)
        pgDSN := "postgres://postgres:secret@postgres:5432/payments_test?sslmode=disable"
        kafkaBrokers := "kafka:29092"
        wiremockURL := "http://wiremock:8080"

        apiCfg := APIContainerConfig{
            PgDSN:             pgDSN,
            KafkaBrokers:      kafkaBrokers,
            KafkaTopics:       suite.Kafka.TopicNames(),
            SilvergateBaseURL: wiremockURL,
            WebhookMode:       opts.WebhookMode, // "kafka" or "sync"
            ProjectRoot:       opts.ProjectRoot,
            Network:           netCfg,
        }
        suite.API, err = NewAPIContainer(ctx, apiCfg)

        ingestCfg := IngestContainerConfig{
            WebhookMode:  opts.IngestWebhookMode(), // "kafka" or "http"
            KafkaBrokers: kafkaBrokers,
            KafkaTopics:  suite.Kafka.TopicNames(),
            APIBaseURL:   "http://api:3000", // Docker DNS
            ProjectRoot:  opts.ProjectRoot,
            Network:       netCfg,
        }
        suite.Ingest, err = NewIngestContainer(ctx, ingestCfg)
    }

    return suite, nil
}
```

### Step 5: E2E test helpers

```go
// integration-test/helpers_test.go

// E2EClient provides typed HTTP helpers for E2E tests.
// IngestURL — send webhooks here (Ingest service)
// APIURL    — query results here (API service)
type E2EClient struct {
    IngestURL string
    APIURL    string
}

func newE2EClient(suite *testinfra.TestSuite) *E2EClient {
    return &E2EClient{
        IngestURL: suite.Ingest.BaseURL,
        APIURL:    suite.API.BaseURL,
    }
}

// Webhook senders — send to Ingest
func (c *E2EClient) SendOrderWebhook(t *testing.T, payload map[string]interface{}) { ... }
func (c *E2EClient) SendChargebackWebhook(t *testing.T, payload map[string]interface{}) { ... }

// Query helpers — query API
func (c *E2EClient) GetOrder(t *testing.T, orderID string) order.Order { ... }
func (c *E2EClient) GetOrders(t *testing.T) []order.Order { ... }
func (c *E2EClient) GetDisputes(t *testing.T) []dispute.Dispute { ... }
func (c *E2EClient) WaitForOrder(t *testing.T, orderID string, maxRetries int) *order.Order { ... }
func (c *E2EClient) WaitForOrderStatus(t *testing.T, orderID, status string, maxRetries int) *order.Order { ... }
func (c *E2EClient) WaitForDisputeByOrderID(t *testing.T, orderID string, maxRetries int) *dispute.Dispute { ... }
func (c *E2EClient) WaitForDisputeStatus(t *testing.T, orderID, status string, maxRetries int) *dispute.Dispute { ... }

// Generic HTTP helpers (reuse existing GET[T]/POST[T])
func GET[T any](t *testing.T, baseUrl, path string, queryPayload any, expectedStatus int) T { ... }
func POST[T any](t *testing.T, baseUrl, path string, payload any, expectedStatus int) T { ... }
```

### Step 6: Kafka mode E2E tests

```go
// integration-test/e2e_kafka_test.go

func TestKafka_CreateOrderFlow(t *testing.T) {
    truncateDB(t)
    client := newE2EClient(suite)

    // Send webhook to INGEST service
    client.SendOrderWebhook(t, orderPayload)

    // Query API service (order processed via Kafka)
    order := client.WaitForOrder(t, "order-1", 40)
    require.NotNil(t, order)
    assert.Equal(t, "created", string(order.Status))
}

func TestKafka_ChargebackFlow(t *testing.T) { ... }
func TestKafka_OrderHoldFlow(t *testing.T) { ... }
func TestKafka_OrderCaptureFlow(t *testing.T) { ... }
func TestKafka_DisputePagination(t *testing.T) { ... }
// ... migrate all 8 existing tests
```

### Step 7: HTTP mode E2E tests

```go
// integration-test/e2e_http_test.go

// HTTP mode tests use a SEPARATE suite (started in TestMain or test-level setup)
// API: WEBHOOK_MODE=sync (no Kafka consumers)
// Ingest: WEBHOOK_MODE=http → POST to API internal endpoints

func TestHTTP_CreateOrderFlow(t *testing.T) {
    // Same test logic as Kafka, but:
    // - No Kafka involved
    // - Ingest sends HTTP to API's /internal/updates/* endpoints
    // - Result is immediate (synchronous), shorter timeouts
}

func TestHTTP_ChargebackFlow(t *testing.T) { ... }
```

**Відкрите питання:** HTTP mode тести вимагають окремий TestSuite (API з WEBHOOK_MODE=sync, Ingest з WEBHOOK_MODE=http). Два варіанти:
1. Окремий `TestMain` (окрема директорія `e2e-http-test/`)
2. Два suite в одному `TestMain` (складніше, але все в одному місці)

Рекомендую варіант 1 — простіше, ізольованіше.

### Step 8: Makefile

```makefile
# E2E tests (Docker-based, Kafka mode)
e2e-test:
	go clean -testcache && go test -tags=integration -v -timeout 5m ./integration-test/...

# E2E tests (Docker-based, HTTP mode)
e2e-test-http:
	go clean -testcache && go test -tags=integration -v -timeout 5m ./e2e-http-test/...
```

## Файли для модифікації

| Файл | Зміни |
|------|-------|
| `internal/shared/testinfra/suite.go` | Add `WithE2E`, network management, API/Ingest container startup |
| `internal/shared/testinfra/postgres.go` | Accept `*NetworkConfig`, add network aliases `["postgres"]` |
| `internal/shared/testinfra/kafka.go` | Accept `*NetworkConfig`, add network aliases `["kafka"]`, expose `TopicNames()` |
| `internal/shared/testinfra/wiremock.go` | Accept `*NetworkConfig`, add network aliases `["wiremock"]` |
| `Makefile` | Add `e2e-test`, `e2e-test-http` targets |

## Файли для створення

| Файл | Призначення |
|------|-------------|
| `internal/shared/testinfra/network.go` | Docker network creation/cleanup |
| `internal/shared/testinfra/api_container.go` | API service container (build from Dockerfile.api) |
| `internal/shared/testinfra/ingest_container.go` | Ingest service container (build from Dockerfile.ingest) |
| `integration-test/main_test.go` | TestMain з Docker E2E suite setup |
| `integration-test/helpers_test.go` | E2EClient, HTTP helpers, wait functions |
| `integration-test/e2e_kafka_test.go` | Kafka mode E2E tests (migrated from integration_test.go) |
| `integration-test/e2e_http_test.go` | HTTP mode E2E tests (new) |

## Файли для видалення

| Файл | Причина |
|------|---------|
| `integration-test/integration_test.go` | Замінений на e2e_kafka_test.go + helpers_test.go + main_test.go |

## Порядок імплементації

1. **Network support** — `network.go` + update constructors (postgres, kafka, wiremock)
2. **API container** — `api_container.go` with health check wait
3. **Ingest container** — `ingest_container.go` with health check wait
4. **Suite update** — `suite.go` WithE2E flow
5. **Test helpers** — `helpers_test.go` (extract from integration_test.go)
6. **Kafka mode tests** — `main_test.go` + `e2e_kafka_test.go` (migrate existing 8 tests)
7. **Delete old test** — remove `integration_test.go`
8. **HTTP mode tests** — `e2e_http_test.go` (new tests, possibly separate directory)
9. **Makefile** — add targets

## Kafka topic name challenge

Поточний підхід: topics створюються в `NewKafka()` з UUID suffix (`test-orders-a1b2c3d4`). API і Ingest контейнери повинні знати ці імена.

Рішення: передати topic names через env vars при створенні API/Ingest контейнерів. `KafkaContainer` вже зберігає `OrdersTopic`, `DisputesTopic` — ці значення передаються в env.

## Міграції в контейнері

API сервіс автоматично застосовує міграції при старті (`ApplyMigrations` в `app.go`). Postgres testcontainer теж застосовує міграції в `NewPostgres()`. Goose idempotent — повторний запуск міграцій = no-op. Проблем немає.

## Trade-offs

**Pros:**
- Максимальний реалізм — тестує Docker images, networking, health checks
- Нуль дублювання bootstrap коду
- Тестує обидва deployment modes (Kafka + HTTP)
- Консистентний підхід — все testcontainers
- Зловлює проблеми з Dockerfile до деплою

**Cons:**
- Повільніший перший запуск (~30-60s build + start)
- Дебаг через container logs (не breakpoints)
- Потребує Docker daemon

**Mitigation:**
- Docker layer caching значно прискорює повторні збірки
- `testcontainers.Container.Logs()` для витягування логів у тест output
- Docker daemon і так потрібен для існуючих testcontainers
