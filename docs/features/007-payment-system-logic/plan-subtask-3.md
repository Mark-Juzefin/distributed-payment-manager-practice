# Plan: Paymanager ↔ Silvergate Integration

## Goal

Paymanager accepts payment requests from clients (card data), authorizes via Silvergate, captures asynchronously, and receives settlement webhooks through Ingest.

## Flow

```
Client                   Paymanager                  Silvergate         Ingest
  |                          |                           |                |
  |-- POST /payments ------->|                           |                |
  |                          |-- POST /api/v1/auth ----->|                |
  |                          |<-- authorized/declined ---|                |
  |<-- 200 {payment} --------|                           |                |
  |                          |-- POST /api/v1/capture -->|  (async)       |
  |                          |                           |                |
  |                          |                           |-- webhook ---->|
  |                          |                           |                |-- Kafka/HTTP -->
  |                          |<-- payment.captured ------|----------------|
  |                          |   (update payment status) |                |
  |                          |                           |                |
  |-- GET /payments/:id ---->|                           |                |
  |<-- {status: captured} ---|                           |                |
```

Client gets 200 immediately after auth (authorized or declined). Capture happens in the background. Client polls `GET /payments/:id` to check final status.

## Changes per Service

### Paymanager

1. **New domain `payment/`** — entity, service, repo interface, state machine, errors
2. **Update `external/silvergate/client.go`** — add `Authorize()` method
3. **New handler** `POST /api/v1/payments` — accepts card data, calls service
4. **New handler** `GET /api/v1/payments/:id` — returns payment status
5. **New consumer** — processes capture webhooks from Silvergate (via Kafka)
6. **Migration** — `payments` table

### Ingest

1. **New handler** `POST /webhooks/silvergate` — accepts webhooks from Silvergate
2. **Extend `Processor` interface** — `ProcessPaymentWebhook()`
3. **New Kafka topic** `webhooks.payments`

## Payment Entity State Machine

```
pending → authorized → capture_pending → captured
  ↓                         ↓
declined              capture_failed
```

## Paymanager API

### POST /api/v1/payments

```json
// Request
{
  "amount": 5000,
  "currency": "USD",
  "card_token": "tok_visa_4242"
}

// Response 200 — authorized (capture in progress)
{
  "id": "pay_abc",
  "status": "authorized",
  "provider_tx_id": "txn_xyz",
  "amount": 5000,
  "currency": "USD"
}

// Response 200 — declined
{
  "id": "pay_abc",
  "status": "declined",
  "decline_reason": "insufficient_funds"
}
```

### GET /api/v1/payments/:id

```json
{
  "id": "pay_abc",
  "status": "captured",
  "amount": 5000,
  "currency": "USD",
  "provider_tx_id": "txn_xyz",
  "created_at": "...",
  "updated_at": "..."
}
```

## DB Schema (Paymanager)

```sql
CREATE TABLE payments (
    id              UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    amount          BIGINT NOT NULL,
    currency        TEXT NOT NULL CHECK (length(currency) = 3),
    card_token      TEXT NOT NULL,
    status          TEXT NOT NULL,
    decline_reason  TEXT,
    provider_tx_id  TEXT,
    created_at      TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at      TIMESTAMPTZ NOT NULL DEFAULT now()
);
```

## Implementation Order

1. [ ] Payment domain (entity, state machine, repo interface, errors)
2. [ ] Migration — `payments` table
3. [ ] PostgreSQL repo
4. [ ] Silvergate client — add `Authorize()` method
5. [ ] Payment service (auth + background capture)
6. [ ] Handlers (POST + GET)
7. [ ] Ingest — webhook handler + processor + Kafka topic
8. [ ] Paymanager consumer — process capture webhooks
9. [ ] Wiring (router, app.go, config)
10. [ ] `http/paymanager.http` file for manual testing
