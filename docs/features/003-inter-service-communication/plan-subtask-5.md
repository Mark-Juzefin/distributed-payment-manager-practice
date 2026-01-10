# Plan: E2E Test Refactoring

## Goal

Переробити інтеграційні тести з in-process `setupTestServer()` на Full E2E:
- Webhook → Ingest (process) → Kafka/HTTP → API (process) → DB
- Підтримка різних `WEBHOOK_MODE` (kafka, http)
- Видалити дублювання коду (~75 рядків)

## Current Problem

`setupTestServer()` в `integration-test/integration_test.go` дублює ~75 рядків з `internal/api/app.go`:
- Repository constructors
- Service constructors
- Handler constructors
- Kafka publisher/consumer setup
- Router setup

При зміні app.go потрібно оновлювати тести вручну.

## Architecture

```
┌─────────────────┐     ┌─────────────────┐     ┌─────────────────┐
│   Test Code     │────▶│  Ingest Service │────▶│   API Service   │
│  (Go test)      │     │  (go run)       │     │   (go run)      │
└─────────────────┘     └─────────────────┘     └─────────────────┘
        │                      │                        │
        │                      ▼                        ▼
        │               ┌─────────────┐         ┌─────────────┐
        └──────────────▶│   Kafka     │         │  PostgreSQL │
                        │(testcontainer)│        │(testcontainer)│
                        └─────────────┘         └─────────────┘
```

## Key Design Decisions

| Decision | Rationale |
|----------|-----------|
| `exec.Command` for processes | No goreman dependency, direct control |
| Health check polling | Wait for services to be ready |
| Testcontainers for infra | Already works, keep for Postgres/Kafka/Wiremock |
| Separate files per mode | Clear organization, parallel development |
| Port ranges per mode | Avoid conflicts: Kafka 13xxx, HTTP 14xxx |

## Implementation Steps

### Step 1: Prerequisites
- [ ] Verify `/health` endpoint exists in Ingest router

### Step 2: Process Management
- [ ] Create `integration-test/process_test.go`
  - `ServiceConfig` struct (mode, ports, DSN, etc.)
  - `startServices(t, cfg)` - start API + Ingest as processes
  - `waitForService(url, timeout)` - health check polling
  - `t.Cleanup()` for automatic shutdown

### Step 3: Test Helpers
- [ ] Create `integration-test/helpers_test.go`
  - `E2EClient` struct with `IngestURL`, `APIURL`
  - `SendOrderWebhook()`, `SendChargebackWebhook()`
  - `WaitForOrder()`, `WaitForDispute()`, etc.
  - Extract from current `integration_test.go`

### Step 4: E2E Tests
- [ ] Create `integration-test/e2e_kafka_test.go`
  - `TestKafkaMode_CreateOrderFlow`
  - `TestKafkaMode_ChargebackFlow`
  - Uses ports 13000-13999

- [ ] Create `integration-test/e2e_http_test.go`
  - `TestHTTPMode_CreateOrderFlow`
  - `TestHTTPMode_ChargebackFlow`
  - Uses ports 14000-14999

### Step 5: Makefile
- [ ] Add `e2e-test` target (all modes)
- [ ] Add `e2e-test-kafka` target
- [ ] Add `e2e-test-http` target

## Files to Create

| File | Purpose |
|------|---------|
| `integration-test/process_test.go` | Service process management |
| `integration-test/helpers_test.go` | Shared E2E helpers |
| `integration-test/e2e_kafka_test.go` | Kafka mode tests |
| `integration-test/e2e_http_test.go` | HTTP mode tests |

## Files to Modify

| File | Changes |
|------|---------|
| `internal/ingest/router.go` | Verify `/health` endpoint exists |
| `Makefile` | Add e2e-test targets |

## Code Examples

### ServiceConfig
```go
type ServiceConfig struct {
    Mode          string // "kafka" or "http"
    APIPort       int
    IngestPort    int
    PgDSN         string
    KafkaBrokers  string
    SilvergateURL string
}
```

### startServices
```go
func startServices(t *testing.T, cfg ServiceConfig) *Services {
    // Start API with env vars
    apiCmd := exec.Command("go", "run", "./cmd/api")
    apiCmd.Env = buildAPIEnv(cfg)
    apiCmd.Start()

    // Wait for API health
    waitForService(apiURL, 30*time.Second)

    // Start Ingest with env vars
    ingestCmd := exec.Command("go", "run", "./cmd/ingest")
    ingestCmd.Env = buildIngestEnv(cfg)
    ingestCmd.Start()

    // Wait for Ingest health
    waitForService(ingestURL, 30*time.Second)

    t.Cleanup(func() { services.Stop() })
    return services
}
```

### Test Example
```go
func TestKafkaMode_CreateOrderFlow(t *testing.T) {
    cfg := ServiceConfig{
        Mode:       "kafka",
        APIPort:    13000,
        IngestPort: 13001,
        PgDSN:      suite.Postgres.DSN,
        // ...
    }

    services := startServices(t, cfg)
    client := &E2EClient{
        IngestURL: services.Ingest.URL,
        APIURL:    services.API.URL,
    }

    // Send webhook to Ingest
    client.SendOrderWebhook(t, payload)

    // Wait for async processing (Kafka)
    found := client.WaitForOrder(t, orderID, 10*time.Second)
    require.True(t, found)
}
```

## Trade-offs

**Pros:**
- True E2E - реальні процеси
- Немає дублювання коду
- Легко тестувати різні конфігурації

**Cons:**
- Повільніший старт (~5-10s per test suite)
- Складніший дебаг (логи в окремих процесах)
