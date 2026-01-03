# Plan: HTTP Sync Mode Implementation

## Goal

Implement HTTP sync mode for inter-service communication: Ingest → HTTP → API.

## Key Architectural Decisions

| Decision | Rationale |
|----------|-----------|
| `apiclient.Client` interface | Extensibility: easy to add gRPC later |
| `internal/shared/dto/` | Shared DTOs for inter-service comm, decoupled from domain |
| "Updates" not "Webhooks" in API | Webhook = Ingest terminology. API receives "updates" |
| Retry with exponential backoff | Resilience for transient failures (no circuit breaker yet) |
| `/internal/updates/*` endpoints | Clear separation of public vs internal API |

## Package Structure

```
internal/
├── api/
│   ├── app.go                      # MODIFY: register internal router
│   ├── internal_router.go          # NEW: /internal/* routes
│   └── handlers/internal/          # NEW: internal handlers package
│       └── updates.go              # Order/dispute update handlers
│
├── ingest/
│   ├── app.go                      # MODIFY: add HTTP mode
│   ├── webhook/
│   │   └── http.go                 # NEW: HTTPSyncProcessor
│   └── apiclient/                  # NEW: API client package
│       ├── client.go               # HTTPClient implementation
│       ├── errors.go               # Error types & mapping
│       └── retry.go                # Retry with backoff
│
├── shared/
│   └── dto/                        # NEW: Shared DTOs
│       ├── order_update.go
│       └── dispute_update.go
│
└── config/config.go                # MODIFY: add HTTP mode config
```

## Key Interfaces

### 1. API Client (New)

```go
// internal/ingest/apiclient/client.go
type Client interface {
    SendOrderUpdate(ctx context.Context, req dto.OrderUpdateRequest) error
    SendDisputeUpdate(ctx context.Context, req dto.DisputeUpdateRequest) error
    Close() error
}
```

### 2. Processor (Existing, unchanged)

```go
// internal/ingest/webhook/processor.go
type Processor interface {
    ProcessOrderWebhook(ctx context.Context, webhook order.PaymentWebhook) error
    ProcessDisputeWebhook(ctx context.Context, webhook dispute.ChargebackWebhook) error
}
```

## API Endpoints

```
POST /internal/updates/orders     → calls orderService.ProcessPaymentWebhook()
POST /internal/updates/disputes   → calls disputeService.ProcessChargeback()
```

## Error Mapping

| HTTP Status | API Client Error | Ingest Response |
|-------------|------------------|-----------------|
| 200-299 | nil | 202 Accepted |
| 404 | ErrNotFound | 404 Not Found |
| 409 | ErrConflict | 200 OK (idempotent) |
| 422 | ErrInvalidStatus | 422 Unprocessable Entity |
| 5xx/timeout | ErrServiceUnavailable | 502/504 Bad Gateway |

## Configuration Changes

```go
// config/config.go - IngestConfig additions
APIBaseURL        string        `env:"API_BASE_URL" envDefault:"http://localhost:3000"`
APITimeout        time.Duration `env:"API_TIMEOUT" envDefault:"10s"`
APIRetryAttempts  int           `env:"API_RETRY_ATTEMPTS" envDefault:"3"`
APIRetryBaseDelay time.Duration `env:"API_RETRY_BASE_DELAY" envDefault:"100ms"`
APIRetryMaxDelay  time.Duration `env:"API_RETRY_MAX_DELAY" envDefault:"5s"`
```

## Implementation Steps

### Step 1: Shared DTOs
- [ ] Create `internal/shared/dto/order_update.go` - OrderUpdateRequest/Response
- [ ] Create `internal/shared/dto/dispute_update.go` - DisputeUpdateRequest/Response

### Step 2: API Internal Endpoints
- [ ] Create `internal/api/handlers/internal/updates.go` - UpdatesHandler
- [ ] Create `internal/api/internal_router.go` - InternalRouter
- [ ] Modify `internal/api/app.go` - register InternalRouter
- [ ] Add unit tests for handlers
- [ ] Add integration tests for endpoints

### Step 3: Ingest API Client
- [ ] Create `internal/ingest/apiclient/errors.go` - error types
- [ ] Create `internal/ingest/apiclient/retry.go` - retry with backoff
- [ ] Create `internal/ingest/apiclient/client.go` - HTTPClient
- [ ] Add unit tests with httptest

### Step 4: HTTP Sync Processor
- [ ] Create `internal/ingest/webhook/http.go` - HTTPSyncProcessor
- [ ] Modify `config/config.go` - add HTTP mode config
- [ ] Modify `internal/ingest/app.go` - create HTTPSyncProcessor when mode="http"
- [ ] Add unit tests with mocked Client

### Step 5: Integration & Testing
- [ ] Add end-to-end integration test
- [ ] Create k6 benchmark: HTTP vs Kafka
- [ ] Update Makefile with `run-http` target
- [ ] Remove old unused SyncProcessor

### Step 6: Documentation
- [ ] Update feature README with completion
- [ ] Add notes about benchmarking results

## Files to Modify

| File | Changes |
|------|---------|
| `internal/api/app.go:64-66` | Add InternalRouter registration after Router |
| `internal/ingest/app.go:27-29` | Add "http" mode handling |
| `config/config.go:10-22` | Add HTTP client config fields |

## Files to Create

| File | Purpose |
|------|---------|
| `internal/shared/dto/order_update.go` | OrderUpdateRequest/Response DTOs |
| `internal/shared/dto/dispute_update.go` | DisputeUpdateRequest/Response DTOs |
| `internal/api/handlers/internal/updates.go` | Internal webhook handlers |
| `internal/api/internal_router.go` | /internal/* route registration |
| `internal/ingest/apiclient/client.go` | HTTP client for API |
| `internal/ingest/apiclient/errors.go` | API client error types |
| `internal/ingest/apiclient/retry.go` | Retry with exponential backoff |
| `internal/ingest/webhook/http.go` | HTTPSyncProcessor |

## Future Extension Points

### gRPC (Subtask 3)
- Implement `apiclient.GRPCClient` satisfying `Client` interface
- Add gRPC server to API service alongside HTTP
- Wire via config: `WEBHOOK_MODE=grpc`

### Circuit Breaker (optional)
- Wrap `Client` with `CircuitBreakerClient` decorator
- Use gobreaker or similar library
- Configurable thresholds and timeout

## Testing Strategy

1. **Unit tests**: Mock `apiclient.Client` for HTTPSyncProcessor
2. **Integration tests**: Real HTTP calls to API in testcontainers
3. **E2E test**: Full flow webhook → Ingest → API → DB
4. **k6 benchmark**: Compare throughput Kafka vs HTTP
