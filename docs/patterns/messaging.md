# Async Messaging

Patterns for moving work off the request path reliably: composable consumer
middleware, an at-least-once Kafka consumer, the outbox → CDC → analytics
pipeline, and an inbox + polling worker. All built so the simple path
(synchronous HTTP) and the complex path (Kafka / inbox) are swappable.

Canonical source: `pkg/messaging/`, `pkg/kafka/`,
`services/ingest/webhook/`, `services/ingest/worker/`, `services/cdc/`,
`services/paymanager/internal/eventstore/`.

---

## 1. Composable consumer middleware (decorators over `MessageHandler`)

A handler is just `func(ctx, key, value []byte) error`. Cross-cutting concerns
(retry, DLQ, metrics) are decorators that wrap one and return the same type, so
they compose.

```go
// pkg/messaging/middleware.go
func WithRetry(handler MessageHandler, cfg RetryConfig) MessageHandler { ... }   // exp backoff + jitter
func WithDLQ(handler MessageHandler, dlq DLQPublisher) MessageHandler { ... }    // dead-letter on failure
func WithMetrics(topic, group string, handler MessageHandler) MessageHandler {...} // duration + counters

// composition — order matters:
h := WithMetrics(topic, group, WithDLQ(WithRetry(businessHandler, cfg), dlq))
```

`WithRetry` does exponential backoff with jitter and is context-aware (aborts on
`ctx.Done()`):

```go
backoff := cfg.InitialBackoff
for attempt := 0; attempt < cfg.MaxAttempts; attempt++ {
    if err = handler(ctx, key, value); err == nil { return nil }
    if attempt < cfg.MaxAttempts-1 {
        jitter := time.Duration(rand.Intn(100)) * time.Millisecond
        select {
        case <-ctx.Done():        return ctx.Err()
        case <-time.After(min(backoff+jitter, cfg.MaxBackoff)):
        }
        backoff *= 2
    }
}
return errors.Join(ErrMaxRetriesExceeded, lastErr)
```

**Why:** each concern is testable alone and opt-in. **Ordering matters:**
`WithMetrics` is outermost so it measures total time *including* retries;
`WithDLQ` is outside `WithRetry` so it only fires after retries are exhausted.

Ref: `pkg/messaging/middleware.go:34` (retry), `:71` (DLQ), `:90` (metrics).

---

## 2. At-least-once consumer (manual commit)

The Kafka consumer commits the offset **only after** the handler succeeds. On
handler error it doesn't commit, so the message is redelivered. Commit uses a
*separate* context so a successful message isn't lost during shutdown.

```go
// pkg/kafka/consumer.go (abridged)
msg, err := c.reader.FetchMessage(ctx)
// ...
if err := handler(msgCtx, msg.Key, msg.Value); err != nil {
    // don't commit → message redelivered on restart
    continue
}
commitCtx, cancel := context.WithTimeout(context.Background(), commitTimeout) // NOT the main ctx
err = c.reader.CommitMessages(commitCtx, msg)
cancel()
// commit failure is non-fatal: redelivery + idempotency will cover it
```

**Why:** explicit commit = at-least-once delivery, the only sane default for
side-effecting consumers. The consequence — duplicate deliveries — is handled by
idempotency downstream (see [transactions.md](transactions.md) §9), not by
pretending exactly-once exists.

Refs: `pkg/kafka/consumer.go:46` (loop), `:113` (correlation-id propagation via headers).

---

## 3. Outbox: write the event in the **same** transaction as the data

To publish an event reliably, write it to an `events` table inside the same
transaction as the business change. A tx-bound store is created by a factory —
the same repo-factory idea as [transactions.md](transactions.md) §3.

```go
// services/paymanager/internal/eventstore/pg.go
func TxStoreFactory(builder squirrel.StatementBuilderType) func(postgres.Executor) Store {
    return func(tx postgres.Executor) Store { return NewPgEventStore(tx, builder) }
}

func (s *PgEventStore) CreateEvent(ctx context.Context, event NewEvent) (*Event, error) {
    // INSERT INTO events (... idempotency_key ...) within the caller's tx
    _, err := s.db.Exec(ctx, query, args...)
    if postgres.IsPgErrorUniqueViolation(err) { return nil, ErrEventAlreadyStored } // idempotent
    // ...
}
```

**Why:** business write and event write commit or roll back together — no "saved
the order but lost the event" gap, and no distributed transaction. The
`idempotency_key` unique index makes re-inserting the same event a no-op. The
event is *published* later by CDC (§4), decoupling write latency from delivery.

Ref: `services/paymanager/internal/eventstore/pg.go:27` (factory), `:33` (insert).

---

## 4. CDC: Postgres WAL → Kafka (the outbox relay)

A standalone worker tails the write-ahead log via logical replication and
republishes inserts to Kafka, keyed by aggregate id. This is what turns the
outbox table into a Kafka stream.

```go
// services/cdc/app.go (shape)
// 1. create logical replication slot + use a publication (pgoutput plugin)
// 2. StartReplication, then streamLoop:
//      - decode InsertMessage from WAL
//      - writer.WriteMessages(kafka.Message{Key: []byte(evt.AggregateID), Value: json})
//      - send periodic StandbyStatusUpdate (heartbeat) to advance the slot
// 3. on connection error → retry with capped backoff
```

```go
writer := &kafka.Writer{
    Addr: kafka.TCP(cfg.KafkaBrokers...), Topic: cfg.KafkaEventsTopic,
    Balancer:     &kafka.Hash{},            // key → partition, preserves per-aggregate order
    BatchTimeout: 10 * time.Millisecond,    // default 1s adds latency
    RequiredAcks: kafka.RequireOne,
}
```

**Why:** the app never blocks on Kafka — it only writes a DB row. CDC is a
separate process that can lag, restart, and replay without touching the write
path. **Trade-off documented in code:** this implementation drops + recreates the
slot on restart (misses events while down) — fine for a log-only listener; for
real delivery you persist the LSN and reuse the slot. Keying by `AggregateID`
preserves per-aggregate ordering across partitions.

Ref: `services/cdc/app.go:90` (runReplication), `:150` (streamLoop), `:219` (handleWALMessage).

---

## 5. Analytics projection (Kafka → OpenSearch)

The analytics service is a consumer (§2) whose handler indexes each event into
OpenSearch — a read model / projection built off the same event stream.

**Why:** search/analytics is a downstream *projection*, not part of the
transactional write. If OpenSearch is down the pipeline degrades (events queue in
Kafka) without affecting payments. Indexing is treated as best-effort.

Ref: `services/analytics/app.go`, `services/analytics/indexer.go`.

---

## 6. Inbox + polling worker (durable webhook ingestion)

The ingest service can persist an incoming webhook to an `inbox` table and return
`202` immediately; a worker polls the table and forwards to the API with retry.

```go
// services/ingest/worker/inbox_worker.go (shape)
for range ticker.C {
    messages, _ := w.repo.FetchPending(ctx, w.cfg.BatchSize)   // claim a batch (SELECT ... FOR UPDATE SKIP LOCKED)
    for _, msg := range messages {
        if err := w.processMessage(ctx, msg); err != nil {
            maxRetries := w.cfg.MaxRetries
            if isPermanentError(err) { maxRetries = 0 }        // 4xx → don't retry, fail fast
            w.repo.MarkFailed(ctx, msg.ID, err.Error(), maxRetries)
        } else {
            w.repo.MarkProcessed(ctx, msg.ID)
        }
    }
}
```

Two details that make it correct:

- **Permanent vs retryable** classification: `400/404/invalid-status` are
  permanent (don't waste retries); everything else is retried.
- **Idempotent forwarding**: a `409 Conflict` from the API means "already
  processed" → treated as success, not an error.

```go
err := w.client.SendOrderUpdate(ctx, req)
if errors.Is(err, apiclient.ErrConflict) { return nil } // already applied — idempotent success
```

**Why:** `FetchPending` using `FOR UPDATE SKIP LOCKED` lets N workers claim
disjoint batches with no double-processing — a DB-backed work queue without a
broker. The inbox decouples "received" from "processed" so a slow/broken API
never drops a webhook.

Refs: `services/ingest/worker/inbox_worker.go:60` (poll), `:99` (dispatch), `:128` (permanent-error check), `services/ingest/repo/inbox/pg_inbox_repo.go` (`SKIP LOCKED` query).

---

## 7. Swappable webhook processor (the complex opt-in seam)

Ingest depends on a `Processor` interface; the binary picks the implementation by
config. Same handlers, three delivery strategies.

```go
// services/ingest/webhook/processor.go
type Processor interface {
    ProcessOrderUpdate(ctx context.Context, req dto.OrderUpdateRequest) error
    ProcessDisputeUpdate(ctx context.Context, req dto.DisputeUpdateRequest) error
    ProcessPaymentWebhook(ctx context.Context, req dto.PaymentWebhookRequest) error
}
```

| Implementation | File | Behaviour |
|----------------|------|-----------|
| HTTP (simple)  | `webhook/http.go`  | forward synchronously to the API |
| Async (Kafka)  | `webhook/async.go` | publish to a topic, return immediately |
| Inbox          | `webhook/inbox.go` | persist to inbox table, worker forwards (§6) |

**Why:** this is the [ddd-structure.md](ddd-structure.md) §6 "complex opt-in"
principle made concrete — Kafka and the inbox are deployment choices, not code
the handler knows about.

Ref: `services/ingest/webhook/processor.go`, siblings `async.go` / `http.go` / `inbox.go`.

---

## 8. DLQ: fail safe, don't block the partition

When retries are exhausted, `WithDLQ` ships the message to a dead-letter queue and
then **returns nil** so the consumer commits the offset — one poison message
can't stall the whole partition. The DLQ publish uses a fresh context so it
completes even during shutdown.

```go
// pkg/messaging/middleware.go
if err := handler(ctx, key, value); err != nil {
    dlqCtx, cancel := context.WithTimeout(context.Background(), dlqPublishTimeout)
    defer cancel()
    _ = dlq.PublishToDLQ(dlqCtx, key, value, err)
    return nil   // commit offset — message is safely parked in the DLQ
}
```

**Why / trade-off:** progress (the partition keeps moving) is chosen over
in-place blocking. The cost is that a DLQ'd message needs a separate
reprocessing path — acceptable, and far better than head-of-line blocking.

Ref: `pkg/messaging/middleware.go:71`.

## Related

- Why duplicates are OK to receive → idempotency in [transactions.md](transactions.md) §9
- Outbox uses the repo-factory pattern → [transactions.md](transactions.md) §3
- Processor/`Acquirer` swappability → [ddd-structure.md](ddd-structure.md) §6
