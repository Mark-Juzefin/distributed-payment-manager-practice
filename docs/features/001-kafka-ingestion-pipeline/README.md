# Step 1: Kafka Ingestion Pipeline

**Status:** Done

## Overview

Async webhook processing via Kafka with microservice extraction and inter-service communication.

**Architecture:** [kafka-architecture.md](kafka-architecture.md)

```
Kafka mode (default):
  Webhook → Ingest (HTTP) → Kafka → API consumer → domain logic

HTTP sync mode:
  Webhook → Ingest → HTTP → API endpoint → domain logic
```

## Phases

### Phase 1: Kafka Integration (Feature 001)

Replaced synchronous webhook processing with Kafka-based async ingestion.

- Webhook handlers publish to Kafka instead of sync processing
- Sync/Kafka mode switch via `WEBHOOK_MODE` env variable
- Testcontainers for test isolation
- Consumer resilience: retry with exponential backoff + jitter, panic recovery, DLQ
- Partition key by `user_id` for sharding-ready architecture
- Idempotency: UNIQUE constraints on events + duplicate detection in handlers

**Detailed Kafka notes:** [kafka-notes.md](kafka-notes.md)

### Phase 2: Ingest Service Extraction (Feature 002)

Created a separate Ingest microservice as a lightweight HTTP → Kafka gateway.

- `cmd/ingest/` binary — lightweight edge service, no DB or domain logic
- Domain errors refactoring — 7 order errors, 2 dispute errors moved to domain layer
- Service-based monorepo: `internal/api/` (primary code owner) + `internal/ingest/` (lightweight)
- Architecture simplification: business logic in `api/`, only `testinfra/` in `shared/`

### Phase 3: Inter-Service Communication (Feature 003)

HTTP sync mode for service-to-service communication between Ingest and API.

- Internal update endpoints: `POST /internal/updates/orders`, `POST /internal/updates/disputes`
- HTTP client in Ingest (`apiclient.Client` interface + `HTTPClient`) with retry logic
- `HTTPSyncProcessor` for sync webhook processing
- Two deployment modes: Kafka (async) and HTTP (sync)
- Progressive approach: HTTP → Protobuf → gRPC (when needed)

---

## Architecture Decision Records

### ADR-1: Consumer placement

**Decision:** Kafka consumer stays in API service, not Ingest.

**Rationale:**
- Retry is natural through Kafka re-delivery (no offset commit → re-process)
- Clear transaction boundaries (1 DB transaction per message)
- Domain determines transient vs permanent errors
- Less distributed complexity

### ADR-2: gRPC for sync mode only

**Decision:** gRPC only for sync mode, not as a layer after Kafka.

**Rationale:**
- Consumer-in-Ingest calling gRPC after Kafka creates complex retries (gRPC timeout ≠ domain error)
- Would require idempotency key on every call
- Distributed transaction problem

### ADR-3: Progressive approach (HTTP → Protobuf → gRPC)

**Decision:** Start with HTTP for service-to-service communication.

**Rationale:**
- HTTP is simpler for debugging (curl, browser dev tools)
- Protobuf separately from gRPC isolates serialization impact for benchmarks
- Each step gives an opportunity for benchmarking
- Lower risk — if gRPC isn't needed, HTTP works

### ADR-4: Internal endpoints

**Decision:** Use `/internal/` prefix for service-to-service endpoints.

**Rationale:**
- Clear separation between public and internal API
- Allows different middleware (auth, rate limiting)
- Standard microservice practice

---

## Deferred Work

### Testing Infrastructure

Plans created but not implemented:

- **E2E Test Refactoring** — process-based test infrastructure, tests for Kafka/HTTP modes
  - Plan: [plan-e2e-test-refactoring.md](plan-e2e-test-refactoring.md)

- **Go-based Load Testing** — order lifecycle + dispute scenarios, VU runner with stats
  - Plan: [plan-load-testing.md](plan-load-testing.md)

### Future: Realistic user_id Lookup

Current approach is simplified — `user_id` added directly to webhook payloads. In reality:
- External providers don't know our internal `user_id`
- Webhook might contain only email or order_id
- Need to lookup `user_id` before Kafka publish or on consume
- Opportunities: cache layer (Redis), lookup service, cross-shard queries

### Graceful Shutdown

DLQ publisher closes before pending messages are sent, causing "io: read/write on closed pipe" errors. Need proper shutdown ordering: stop consumers → flush DLQ → close publishers.

---

## Key Implementation Notes

### Idempotency

Kafka guarantees at-least-once delivery. UNIQUE constraints in DB protect against duplicates:
```sql
-- order_events
UNIQUE (order_id, provider_event_id)
-- dispute_events (partitioned!)
UNIQUE (dispute_id, provider_event_id, created_at)
```

Handler logic: duplicate → DB rejects → handler returns nil → offset commits → consumer progresses.

### Partition Key: user_id vs order_id

| Aspect | order_id | user_id |
|--------|----------|---------|
| Ordering | Per order (too granular) | Per user (preserves causality) |
| Sharding | Doesn't match DB shards | Ideal for hash(user_id) sharding |
| Load | Hot orders = hot partition | Even distribution |
| Queries | Cross-shard for user data | All user data on one shard |

### Retry/DLQ Middleware Chain

```
WithMetrics → WithRetry → WithDLQ → Handler
```

- Retry: exponential backoff (100ms → 200ms → 400ms) + jitter, max 3 attempts
- DLQ: after max retries, publish to DLQ topic, commit offset anyway (prevents poison message blocking)

### Architecture Lesson

Extensible architecture from the start made it easy to add DLQ wrapper, retry middleware, mode switching via env, and consumer scaling later.

---

## Key Files

| File | Purpose |
|------|---------|
| `internal/api/messaging/middleware.go` | Retry + DLQ + Metrics middleware |
| `internal/api/messaging/runner.go` | Worker lifecycle manager |
| `internal/api/external/kafka/consumer.go` | Kafka consumer (segmentio/kafka-go) |
| `internal/api/external/kafka/publisher.go` | Kafka publisher with Hash balancer |
| `internal/api/webhook/processor.go` | Processor interface |
| `internal/api/webhook/async.go` | AsyncProcessor (HTTP → Kafka) |
| `internal/api/webhook/sync.go` | SyncProcessor (HTTP → Service) |
| `internal/api/consumers/order.go` | Order message handler |
| `internal/api/consumers/dispute.go` | Dispute message handler |
| `internal/api/handlers/updates/` | Internal update endpoints (sync mode) |
| `internal/ingest/apiclient/` | HTTP client for sync mode |
| `internal/ingest/webhook/http.go` | HTTPSyncProcessor |
