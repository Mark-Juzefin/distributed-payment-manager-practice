# Plan: Refund in Silvergate

## Goal

Add refund support to Silvergate PSP. Supports full and partial refunds on captured transactions. Async processing (202 + webhook) like capture.

## Flow

```
Merchant                    Silvergate                  Bank (mock)
   |                            |                          |
   |--- POST /api/v1/refund -->|                          |
   |<-- 202 accepted ----------|                          |
   |                            |--- refund request ----->|
   |                            |<-- approved/denied -----|
   |                            |                          |
   |<-- webhook (refunded) ----|                          |
```

## API

### POST /api/v1/refund

```json
// Request
{
  "transaction_id": "txn_abc",
  "amount": 2000,
  "idempotency_key": "ref_001"
}

// Response 202
{
  "refund_id": "ref_xyz",
  "transaction_id": "txn_abc",
  "amount": 2000,
  "status": "refund_pending"
}
```

Amount can be less than original (partial) or equal (full). Multiple partial refunds allowed until total_refunded == original amount.

### Webhook

```json
{
  "event": "transaction.refunded",
  "transaction_id": "txn_abc",
  "refund_id": "ref_xyz",
  "status": "refunded",
  "amount": 2000,
  "currency": "USD"
}
```

## DB Schema

New table (one transaction can have many refunds):

```sql
CREATE TABLE refunds (
    id              UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    transaction_id  UUID NOT NULL REFERENCES transactions(id),
    amount          BIGINT NOT NULL,
    status          TEXT NOT NULL CHECK (status IN ('refund_pending', 'refunded', 'refund_failed')),
    idempotency_key TEXT,
    created_at      TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at      TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE UNIQUE INDEX idx_refunds_idempotency
    ON refunds(transaction_id, idempotency_key)
    WHERE idempotency_key IS NOT NULL;
```

Transaction gets new fields: `refunded_amount BIGINT DEFAULT 0`, new status `refunded` (when fully refunded), `partially_refunded`.

## Transaction State Machine Update

```
captured → partially_refunded → refunded (when refunded_amount == amount)
captured → refunded (full refund in one go)
```

## Refund State Machine

```
refund_pending → refunded
refund_pending → refund_failed
```

## Acquirer Interface

```go
Refund(ctx context.Context, txID string, amount int64) (RefundResult, error)
```

Mock: 95% success rate, same delay as settle.

## Validation

- Transaction must be `captured` or `partially_refunded`
- `refunded_amount + requested_amount <= transaction.amount`
- Idempotency via `(transaction_id, idempotency_key)` unique index

## Implementation Order

1. [ ] Migration: `refunds` table + `refunded_amount` column + new statuses on transactions
2. [ ] Refund entity + state machine
3. [ ] Acquirer: `Refund()` method + mock
4. [ ] Refund repo
5. [ ] Service: refund logic (validate, create refund, async bank call, webhook)
6. [ ] Handler: `POST /api/v1/refund`
7. [ ] Webhook: `transaction.refunded` / `transaction.refund_failed` events
8. [ ] Router + wiring
9. [ ] Update `http/silvergate.http`
