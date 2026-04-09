# Plan: Silvergate Service — Auth & Capture

## Goal

Create a standalone Go service `services/silvergate` — a PSP (Payment Service Provider) that accepts auth/capture requests from merchants, stores transactions in its own DB schema, and sends webhooks back about async results.

## Current State

Silvergate exists only as an HTTP client in paymanager (`external/silvergate/client.go`) + Wiremock stubs. No actual service.

## Auth & Settle Flow

```
Merchant                    Silvergate                  Bank (mock)
   |                            |                          |
   |--- POST /api/v1/auth ---->|                          |
   |                            |--- authorize ---------->|
   |                            |<-- approved/declined ---|
   |<-- 200 {status, tx_id} ---|                          |
   |                            |                          |
   |--- POST /api/v1/capture ->|                          |
   |<-- 202 accepted ----------|                          |
   |                            |--- settle ------------->|
   |                            |<-- settled/failed ------|
   |                            |                          |
   |<-- webhook (settled) -----|                          |
```

- **Auth** — synchronous. Merchant gets immediate response (authorized/declined).
- **Capture** — asynchronous. Merchant gets 202, then webhook when bank confirms settlement.

## Architectural Decisions

| Question | Decision | Why |
|----------|----------|-----|
| Separate DB? | Separate PostgreSQL schema (`silvergate`) | Service isolation, realistic |
| Auth sync/async? | Synchronous | Real PSPs: fund hold is fast |
| Capture sync/async? | Async — 202 + webhook | Realistic: settlement takes time |
| Bank mock | `Acquirer` interface with in-memory impl | Simple, configurable approve rate |
| Webhook delivery | Goroutine + HTTP POST | Simple, can add retry/queue later |
| Framework | Gin | Consistency with other services |
| Amount type | `int64` (cents) | Fintech standard, no float issues |
| merchant_id | Stored per transaction | Supports multi-merchant |
| Port | 3002 | After paymanager:3000, ingest:3001 |
| card_token | String, not validated | Stored for realism, mock bank ignores |

## DB Schema

```sql
CREATE SCHEMA IF NOT EXISTS silvergate;

CREATE TABLE silvergate.transactions (
    id              UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    merchant_id     TEXT NOT NULL,
    order_ref       TEXT NOT NULL,
    amount          BIGINT NOT NULL,
    currency        TEXT NOT NULL,
    card_token      TEXT NOT NULL,
    status          TEXT NOT NULL,
    decline_reason  TEXT,
    idempotency_key TEXT,
    created_at      TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at      TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE UNIQUE INDEX idx_transactions_idempotency
    ON silvergate.transactions(merchant_id, idempotency_key)
    WHERE idempotency_key IS NOT NULL;
```

## Service Structure

```
services/silvergate/
├── cmd/main.go
├── app.go
├── config/config.go
├── router.go
├── migrations/
├── domain/
│   └── transaction/
│       ├── entity.go      # Transaction entity, status state machine
│       ├── service.go      # Auth & capture business logic
│       ├── repo.go         # Repository interface
│       └── errors.go       # Domain errors
├── acquirer/
│   ├── port.go             # Acquirer interface (bank)
│   └── mock_acquirer.go    # In-memory mock (configurable approve rate)
├── webhook/
│   └── sender.go           # HTTP webhook sender
├── handlers/
│   ├── auth.go             # POST /api/v1/auth
│   └── capture.go          # POST /api/v1/capture
└── repo/
    └── pg_transaction.go   # PostgreSQL repo
```

## API

### POST /api/v1/auth

```json
// Request
{
  "merchant_id": "merchant_1",
  "order_id": "ord_123",
  "amount": 5000,
  "currency": "USD",
  "card_token": "tok_visa_4242"
}

// Response 200 — authorized
{
  "transaction_id": "txn_abc",
  "status": "authorized",
  "order_id": "ord_123"
}

// Response 200 — declined
{
  "transaction_id": "txn_abc",
  "status": "declined",
  "decline_reason": "insufficient_funds"
}
```

### POST /api/v1/capture

```json
// Request
{
  "transaction_id": "txn_abc",
  "amount": 5000,
  "idempotency_key": "cap_key_1"
}

// Response 202
{
  "transaction_id": "txn_abc",
  "status": "capture_pending"
}

// Webhook (async) → merchant callback URL
{
  "event": "transaction.captured",
  "transaction_id": "txn_abc",
  "order_id": "ord_123",
  "status": "captured",
  "amount": 5000,
  "currency": "USD"
}
```

## Transaction State Machine

```
(new) → authorized → captured
  ↓         ↓
declined   capture_failed
```

## Acquirer (Bank) Interface

```go
type Acquirer interface {
    Authorize(ctx context.Context, amount int64, currency, cardToken string) (AuthResult, error)
    Settle(ctx context.Context, txID string, amount int64) (SettleResult, error)
}
```

Mock: 90% approve rate for auth, 95% for settle. Configurable via env.

## Implementation Order

1. [ ] Go module skeleton: `go.mod`, `config`, `cmd/main.go`, `app.go`
2. [ ] Domain: `transaction/entity.go`, state machine, `repo.go` interface, errors
3. [ ] Migration: `silvergate.transactions` table
4. [ ] Repo: PostgreSQL implementation
5. [ ] Acquirer: interface + mock implementation
6. [ ] Service: auth & capture business logic
7. [ ] Handlers: HTTP endpoints
8. [ ] Webhook sender: async merchant notification
9. [ ] Router + wiring in `app.go`
10. [ ] Infra: docker-compose port, env files, Procfile, go.work
