# Plan: Subtask 4 - Consumer Resilience

## Goal

Add retry with exponential backoff, panic recovery, and Dead Letter Queue for poison messages.

## Current State

`internal/messaging/middleware.go` already created with:
- `WithRetry()` - exponential backoff + jitter
- `WithDLQ()` - dead letter queue wrapper
- `DLQPublisher` interface

`internal/messaging/runner.go` already has panic recovery at worker level.

## Remaining Tasks

### 1. DLQ Publisher Implementation

Create `internal/external/kafka/dlq_publisher.go`:
```go
type DLQPublisher struct {
    writer *kafka.Writer
    logger *logger.Logger
}

func (p *DLQPublisher) PublishToDLQ(ctx context.Context, key, value []byte, err error) error {
    // Add error header, publish to DLQ topic
}
```

### 2. Config for DLQ Topics

Add to `config/config.go`:
```go
KafkaOrdersDLQTopic   string `env:"KAFKA_ORDERS_DLQ_TOPIC" envDefault:"orders.webhooks.dlq"`
KafkaDisputesDLQTopic string `env:"KAFKA_DISPUTES_DLQ_TOPIC" envDefault:"disputes.webhooks.dlq"`
```

### 3. App Wiring

Update `internal/app/app.go`:
```go
// Create DLQ publishers
orderDLQPub := kafka.NewDLQPublisher(logger, cfg.KafkaBrokers, cfg.KafkaOrdersDLQTopic)
disputeDLQPub := kafka.NewDLQPublisher(logger, cfg.KafkaBrokers, cfg.KafkaDisputesDLQTopic)

// Wrap handlers with retry + DLQ
orderHandler := messaging.WithDLQ(
    messaging.WithRetry(orderController.HandleMessage, messaging.DefaultRetryConfig()),
    orderDLQPub,
)
```

### 4. Verify Panic Recovery

Panic recovery already exists in `runner.go:36-39`. Verify it works correctly.

## Files to Modify

| File | Changes |
|------|---------|
| `internal/external/kafka/dlq_publisher.go` | New - DLQ publisher |
| `config/config.go` | Add DLQ topic config |
| `internal/app/app.go` | Wire middleware |
| `docker-compose.yml` | Add DLQ topics (optional) |

## Order of Implementation

1. Add DLQ topic config
2. Create DLQ publisher
3. Wire middleware in app.go
4. Test manually
