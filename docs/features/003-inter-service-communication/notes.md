# Notes: Inter-Service Communication (Feature 003)

## Key Files

| File | Purpose |
|------|---------|
| `internal/ingest/webhook/processor.go` | Processor interface (shared by all modes) |
| `internal/ingest/webhook/async.go` | AsyncProcessor (HTTP → Kafka) |
| `internal/ingest/webhook/http.go` | HTTPSyncProcessor (HTTP → HTTP) |
| `internal/ingest/apiclient/client.go` | API client interface + HTTP implementation |
| `internal/ingest/apiclient/errors.go` | Error types for inter-service communication |
| `internal/ingest/apiclient/retry.go` | Retry with exponential backoff |
| `internal/api/handlers/internal/updates.go` | Internal API handlers for updates |
| `internal/api/internal_router.go` | /internal/* route registration |
| `internal/shared/dto/` | Shared DTOs for inter-service communication |

---

## Architecture Overview

### Three Communication Modes

```
Kafka mode (async, production):
  Webhook → Ingest → Kafka → API consumer → domain logic

HTTP sync mode:
  Webhook → Ingest → HTTP → API /internal/updates → domain logic

gRPC sync mode (future):
  Webhook → Ingest → gRPC → API server → domain logic
```

### Key Abstractions

**Processor interface** - unchanged from Feature 001:
```go
type Processor interface {
    ProcessOrderWebhook(ctx context.Context, webhook order.PaymentWebhook) error
    ProcessDisputeWebhook(ctx context.Context, webhook dispute.ChargebackWebhook) error
}
```

**Client interface** - new abstraction for inter-service communication:
```go
type Client interface {
    SendOrderUpdate(ctx context.Context, req dto.OrderUpdateRequest) error
    SendDisputeUpdate(ctx context.Context, req dto.DisputeUpdateRequest) error
    Close() error
}
```

---

## HTTP Sync Mode Details

### Request/Response Flow

```
1. External provider sends webhook to Ingest
   POST /webhooks/payments/orders {provider_event_id, order_id, status...}

2. Ingest handler parses JSON into order.PaymentWebhook

3. HTTPSyncProcessor converts to DTO:
   order.PaymentWebhook → dto.OrderUpdateRequest

4. HTTPClient sends to API:
   POST /internal/updates/orders
   - Retry on 5xx/timeout (exponential backoff)
   - Map response status to domain errors

5. API internal handler:
   - Parses dto.OrderUpdateRequest
   - Converts to order.PaymentWebhook
   - Calls orderService.ProcessPaymentWebhook()
   - Returns appropriate HTTP status

6. Ingest returns response to external provider
```

### Error Handling

| Scenario | API Response | Client Error | Ingest Response |
|----------|--------------|--------------|-----------------|
| Success | 200 OK | nil | 202 Accepted |
| Duplicate event | 409 Conflict | ErrConflict | 200 OK (idempotent) |
| Order not found | 404 Not Found | ErrNotFound | 404 Not Found |
| Invalid transition | 422 Unprocessable | ErrInvalidStatus | 422 Unprocessable |
| API down | 5xx / timeout | ErrServiceUnavailable | 502/504 Gateway |

### Retry Strategy

```go
RetryConfig{
    MaxAttempts:  3,
    BaseDelay:    100ms,   // → 200ms → 400ms (exponential)
    MaxDelay:     5s,
    // Jitter: +/- 25% to avoid thundering herd
}
```

Only retries on `ErrServiceUnavailable` (transient errors).

---

## Terminology

| Ingest Service | API Service |
|----------------|-------------|
| Webhook | Update |
| ProcessOrderWebhook | ProcessOrderUpdate |
| /webhooks/payments/orders | /internal/updates/orders |

Rationale: "Webhook" is external provider terminology. Internally, we receive "state updates".

---

## Configuration

### Ingest Service

```bash
# Mode selection
WEBHOOK_MODE=http          # "kafka" or "http"

# HTTP mode settings
API_BASE_URL=http://localhost:3000
API_TIMEOUT=10s
API_RETRY_ATTEMPTS=3
API_RETRY_BASE_DELAY=100ms
API_RETRY_MAX_DELAY=5s
```

---

## Benchmarking

### k6 Test Scenarios

TODO: Add benchmark results after implementation

| Mode | RPS | p50 latency | p99 latency | Notes |
|------|-----|-------------|-------------|-------|
| Kafka | | | | Async, eventual consistency |
| HTTP | | | | Sync, immediate consistency |

---

## Future: Circuit Breaker

Currently not implemented, but architecture supports adding it as a decorator:

```go
type CircuitBreakerClient struct {
    client Client
    cb     *gobreaker.CircuitBreaker
}

func (c *CircuitBreakerClient) SendOrderUpdate(ctx context.Context, req dto.OrderUpdateRequest) error {
    _, err := c.cb.Execute(func() (interface{}, error) {
        return nil, c.client.SendOrderUpdate(ctx, req)
    })
    return err
}
```

**Circuit Breaker pattern:**
- Prevents cascading failures when downstream service is unhealthy
- States: Closed → Open → Half-Open
- Opens circuit after N consecutive failures
- Periodically allows test requests to check recovery

---

## Future: gRPC Mode

Extension path for Subtask 3:

1. Define proto file with `UpdatesService`
2. Implement `GRPCClient` satisfying `Client` interface
3. Add gRPC server to API service
4. Wire via `WEBHOOK_MODE=grpc`

---

## Learnings

<!--
Add your notes here:
- What was challenging
- What could be improved
- Ideas for the future
-->

