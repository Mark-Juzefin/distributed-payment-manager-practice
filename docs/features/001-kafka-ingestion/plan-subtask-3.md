# План: Testcontainers для ізоляції тестів

## Мета

Перевести інтеграційні тести на testcontainers для повної ізоляції. Кожен test suite отримує власну інфраструктуру (Kafka, PostgreSQL, Wiremock). Docker-compose залишається тільки для локальної розробки.

## Поточний стан

| Компонент | Зараз | Після |
|-----------|-------|-------|
| `dispute_eventsink` tests | testcontainer (PG) | без змін |
| `order_eventsink` tests | testcontainer (PG) | без змін |
| `integration-test/` | docker-compose | testcontainers |
| docker-compose | tests + dev | тільки dev |

## Архітектурні рішення

| Питання | Рішення | Чому |
|---------|---------|------|
| Де розмістити testinfra? | `internal/testinfra/` | Shared між усіма тестами, internal бо не експортуємо |
| Як ізолювати Kafka тести? | Унікальні топіки per test run | Не потребує LastOffset, auto.create.topics працює |
| Чи потрібен OpenSearch в тестах? | Ні, mock або skip | OpenSearch optional, не критичний для бізнес-логіки |
| Wiremock чи httptest? | Wiremock container | Вже є mappings, консистентність з dev |

## Структура пакету testinfra

```
internal/
└── testinfra/
    ├── postgres.go      # PostgreSQL container + migrations
    ├── kafka.go         # Kafka container (KRaft mode, no Zookeeper)
    ├── wiremock.go      # Wiremock container для Silvergate mock
    └── suite.go         # TestSuite - композиція всіх сервісів
```

## Компонент 1: PostgreSQL Container

**Файл:** `internal/testinfra/postgres.go`

```go
package testinfra

import (
    "context"
    "fmt"
    "time"

    "TestTaskJustPay/internal/app"
    "TestTaskJustPay/pkg/postgres"

    "github.com/docker/go-connections/nat"
    "github.com/testcontainers/testcontainers-go"
    "github.com/testcontainers/testcontainers-go/wait"
)

type PostgresContainer struct {
    Container testcontainers.Container
    Pool      *postgres.Postgres
    DSN       string
}

func NewPostgres(ctx context.Context) (*PostgresContainer, error) {
    req := testcontainers.ContainerRequest{
        Image: "pg17-partman:local",
        Env: map[string]string{
            "POSTGRES_USER":     "postgres",
            "POSTGRES_PASSWORD": "secret",
            "POSTGRES_DB":       "payments_test",
        },
        ExposedPorts: []string{"5432/tcp"},
        WaitingFor: wait.ForSQL("5432/tcp", "postgres",
            func(host string, port nat.Port) string {
                return fmt.Sprintf("postgres://postgres:secret@%s:%s/payments_test?sslmode=disable", host, port.Port())
            },
        ).WithStartupTimeout(60 * time.Second),
    }

    container, err := testcontainers.GenericContainer(ctx,
        testcontainers.GenericContainerRequest{
            ContainerRequest: req,
            Started:          true,
        },
    )
    if err != nil {
        return nil, fmt.Errorf("failed to start postgres container: %w", err)
    }

    host, _ := container.Host(ctx)
    port, _ := container.MappedPort(ctx, "5432/tcp")
    dsn := fmt.Sprintf("postgres://postgres:secret@%s:%s/payments_test?sslmode=disable", host, port.Port())

    pool, err := postgres.New(dsn, postgres.MaxPoolSize(10))
    if err != nil {
        container.Terminate(ctx)
        return nil, fmt.Errorf("failed to create postgres pool: %w", err)
    }

    // Apply migrations
    if err := app.ApplyMigrations(dsn, app.MIGRATION_FS); err != nil {
        pool.Close()
        container.Terminate(ctx)
        return nil, fmt.Errorf("failed to apply migrations: %w", err)
    }

    return &PostgresContainer{
        Container: container,
        Pool:      pool,
        DSN:       dsn,
    }, nil
}

func (c *PostgresContainer) Cleanup(ctx context.Context) {
    if c.Pool != nil {
        c.Pool.Close()
    }
    if c.Container != nil {
        c.Container.Terminate(ctx)
    }
}

// Truncate очищає всі таблиці (для ізоляції між тестами)
func (c *PostgresContainer) Truncate(ctx context.Context) error {
    _, err := c.Pool.Pool.Exec(ctx,
        "TRUNCATE TABLE dispute_events, disputes, order_events, orders, evidence CASCADE")
    return err
}
```

## Компонент 2: Kafka Container (KRaft)

**Файл:** `internal/testinfra/kafka.go`

Використовуємо KRaft mode (без Zookeeper) - простіше і швидше для тестів.

```go
package testinfra

import (
    "context"
    "fmt"

    "github.com/google/uuid"
    "github.com/testcontainers/testcontainers-go/modules/kafka"
)

type KafkaContainer struct {
    Container    *kafka.KafkaContainer
    Brokers      []string
    OrdersTopic  string
    DisputesTopic string
    OrdersGroup  string
    DisputesGroup string
}

func NewKafka(ctx context.Context) (*KafkaContainer, error) {
    container, err := kafka.Run(ctx,
        "confluentinc/confluent-local:7.5.0",
        kafka.WithClusterID("test-cluster"),
    )
    if err != nil {
        return nil, fmt.Errorf("failed to start kafka container: %w", err)
    }

    brokers, err := container.Brokers(ctx)
    if err != nil {
        container.Terminate(ctx)
        return nil, fmt.Errorf("failed to get brokers: %w", err)
    }

    // Унікальні топіки та групи per test run
    // Kafka auto-creates topics on first produce
    suffix := uuid.New().String()[:8]

    return &KafkaContainer{
        Container:     container,
        Brokers:       brokers,
        OrdersTopic:   fmt.Sprintf("test-orders-%s", suffix),
        DisputesTopic: fmt.Sprintf("test-disputes-%s", suffix),
        OrdersGroup:   fmt.Sprintf("test-group-orders-%s", suffix),
        DisputesGroup: fmt.Sprintf("test-group-disputes-%s", suffix),
    }, nil
}

func (c *KafkaContainer) Cleanup(ctx context.Context) {
    if c.Container != nil {
        c.Container.Terminate(ctx)
    }
}
```

**Чому KRaft:**
- Не потрібен окремий Zookeeper container
- Швидший startup (~2-3 сек замість ~5-7 сек)
- Сучасний підхід (Zookeeper deprecated в Kafka 3.5+)

## Компонент 3: Wiremock Container

**Файл:** `internal/testinfra/wiremock.go`

```go
package testinfra

import (
    "context"
    "fmt"
    "path/filepath"

    "github.com/testcontainers/testcontainers-go"
    "github.com/testcontainers/testcontainers-go/wait"
)

type WiremockContainer struct {
    Container testcontainers.Container
    BaseURL   string
}

func NewWiremock(ctx context.Context, mappingsPath string) (*WiremockContainer, error) {
    absPath, err := filepath.Abs(mappingsPath)
    if err != nil {
        return nil, fmt.Errorf("failed to get absolute path: %w", err)
    }

    req := testcontainers.ContainerRequest{
        Image:        "wiremock/wiremock:latest",
        ExposedPorts: []string{"8080/tcp"},
        WaitingFor:   wait.ForHTTP("/__admin/mappings").WithPort("8080/tcp"),
        Cmd:          []string{"--global-response-templating", "--disable-gzip", "--verbose"},
        Mounts: testcontainers.Mounts(
            testcontainers.BindMount(absPath, "/home/wiremock/mappings"),
        ),
    }

    container, err := testcontainers.GenericContainer(ctx,
        testcontainers.GenericContainerRequest{
            ContainerRequest: req,
            Started:          true,
        },
    )
    if err != nil {
        return nil, fmt.Errorf("failed to start wiremock container: %w", err)
    }

    host, _ := container.Host(ctx)
    port, _ := container.MappedPort(ctx, "8080/tcp")
    baseURL := fmt.Sprintf("http://%s:%s", host, port.Port())

    return &WiremockContainer{
        Container: container,
        BaseURL:   baseURL,
    }, nil
}

func (c *WiremockContainer) Cleanup(ctx context.Context) {
    if c.Container != nil {
        c.Container.Terminate(ctx)
    }
}
```

## Компонент 4: TestSuite (композиція)

**Файл:** `internal/testinfra/suite.go`

```go
package testinfra

import (
    "context"
    "fmt"
    "sync"
)

type TestSuite struct {
    Postgres *PostgresContainer
    Kafka    *KafkaContainer
    Wiremock *WiremockContainer
}

type SuiteOptions struct {
    WithKafka    bool
    WithWiremock bool
    MappingsPath string // для Wiremock
}

// NewTestSuite створює всю інфраструктуру для тестів
// Контейнери запускаються паралельно для швидкості
func NewTestSuite(ctx context.Context, opts SuiteOptions) (*TestSuite, error) {
    suite := &TestSuite{}
    var wg sync.WaitGroup
    errCh := make(chan error, 3)

    // PostgreSQL (завжди потрібен)
    wg.Add(1)
    go func() {
        defer wg.Done()
        pg, err := NewPostgres(ctx)
        if err != nil {
            errCh <- fmt.Errorf("postgres: %w", err)
            return
        }
        suite.Postgres = pg
    }()

    // Kafka (опціонально)
    if opts.WithKafka {
        wg.Add(1)
        go func() {
            defer wg.Done()
            k, err := NewKafka(ctx)
            if err != nil {
                errCh <- fmt.Errorf("kafka: %w", err)
                return
            }
            suite.Kafka = k
        }()
    }

    // Wiremock (опціонально)
    if opts.WithWiremock {
        wg.Add(1)
        go func() {
            defer wg.Done()
            w, err := NewWiremock(ctx, opts.MappingsPath)
            if err != nil {
                errCh <- fmt.Errorf("wiremock: %w", err)
                return
            }
            suite.Wiremock = w
        }()
    }

    wg.Wait()
    close(errCh)

    // Збираємо помилки
    var errs []error
    for err := range errCh {
        errs = append(errs, err)
    }
    if len(errs) > 0 {
        suite.Cleanup(ctx) // cleanup partially started containers
        return nil, fmt.Errorf("failed to start containers: %v", errs)
    }

    return suite, nil
}

func (s *TestSuite) Cleanup(ctx context.Context) {
    if s.Wiremock != nil {
        s.Wiremock.Cleanup(ctx)
    }
    if s.Kafka != nil {
        s.Kafka.Cleanup(ctx)
    }
    if s.Postgres != nil {
        s.Postgres.Cleanup(ctx)
    }
}
```

## Зміни в integration-test/integration_test.go

**Основні зміни:**

```go
var suite *testinfra.TestSuite

func TestMain(m *testing.M) {
    ctx := context.Background()

    var err error
    suite, err = testinfra.NewTestSuite(ctx, testinfra.SuiteOptions{
        WithKafka:    true,
        WithWiremock: true,
        MappingsPath: "mappings", // відносний шлях від integration-test/
    })
    if err != nil {
        panic(fmt.Sprintf("Failed to start test suite: %v", err))
    }

    code := m.Run()

    suite.Cleanup(ctx)
    os.Exit(code)
}

func setupTestServer(t *testing.T) *httptest.Server {
    // Використовуємо дані з suite замість env vars
    cfg := config.Config{
        PgURL:     suite.Postgres.DSN,
        PgPoolMax: 10,
        LogLevel:  "debug",

        SilvergateBaseURL:                 suite.Wiremock.BaseURL,
        SilvergateSubmitRepresentmentPath: "/representment",
        SilvergateCapturePath:             "/capture",

        WebhookMode: "kafka",
        KafkaBrokers:               suite.Kafka.Brokers,
        KafkaOrdersTopic:           suite.Kafka.OrdersTopic,
        KafkaDisputesTopic:         suite.Kafka.DisputesTopic,
        KafkaOrdersConsumerGroup:   suite.Kafka.OrdersGroup,
        KafkaDisputesConsumerGroup: suite.Kafka.DisputesGroup,

        // OpenSearch не потрібен для цих тестів
        OpensearchUrls:          []string{},
        OpensearchIndexDisputes: "test-disputes",
        OpensearchIndexOrders:   "test-orders",
    }

    // ... решта setup коду
}
```

## Рефакторинг існуючих testcontainers

Після створення `testinfra` пакету, оновити:
- `internal/repo/dispute_eventsink/integration_test.go` - використовувати `testinfra.NewPostgres()`
- `internal/repo/order_eventsink/integration_test.go` - використовувати `testinfra.NewPostgres()`

Це зменшить дублювання коду.

## Файли для створення/модифікації

| Файл | Дія |
|------|-----|
| `internal/testinfra/postgres.go` | NEW |
| `internal/testinfra/kafka.go` | NEW |
| `internal/testinfra/wiremock.go` | NEW |
| `internal/testinfra/suite.go` | NEW |
| `integration-test/integration_test.go` | MODIFY: перейти на testinfra |
| `internal/repo/dispute_eventsink/integration_test.go` | MODIFY: використати testinfra |
| `internal/repo/order_eventsink/integration_test.go` | MODIFY: використати testinfra |

## Залежності

Додати в `go.mod`:
```
github.com/testcontainers/testcontainers-go/modules/kafka
```

(базовий testcontainers вже є)

## Порядок імплементації

1. Створити `internal/testinfra/postgres.go` (витягнути з існуючих тестів)
2. Створити `internal/testinfra/kafka.go` (новий)
3. Створити `internal/testinfra/wiremock.go` (новий)
4. Створити `internal/testinfra/suite.go` (композиція)
5. Мігрувати `integration-test/integration_test.go` на testinfra
6. Видалити `isKafkaMode` та dual-mode логіку (завжди kafka в тестах)
7. Рефакторинг `dispute_eventsink` та `order_eventsink` тестів на testinfra
8. Оновити Makefile: `integration-test` більше не потребує `start_containers`

## Makefile зміни

```makefile
# Було:
integration-test: start_containers
	go test -tags=integration -v ./integration-test/...

# Стане:
integration-test:
	go test -tags=integration -v ./integration-test/...
```

Docker-compose залишається для `make run-dev`.

## Що залишається в docker-compose

Docker-compose використовується тільки для локальної розробки (`make run-dev`):
- PostgreSQL (для app)
- Kafka + Zookeeper (для app)
- Kafka UI (для debugging)
- OpenSearch cluster (для analytics)
- Wiremock (для manual testing)

Тести повністю self-contained через testcontainers.
