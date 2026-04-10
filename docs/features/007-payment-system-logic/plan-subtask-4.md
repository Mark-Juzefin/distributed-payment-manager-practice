# Plan: Void — capture_delay + void endpoint

## Goal

Add configurable capture delay to payments and void functionality. Client can create a payment with a delay window, then void it before capture happens.

## Flow

```
# Instant capture (default, capture_delay=0)
POST /payments { amount: 5000 }
  → auth → capture immediately

# Delayed capture (client wants a cancellation window)  
POST /payments { amount: 5000, capture_delay: "5m" }
  → auth → status: authorized, capture_at: now+5m
  → background: sleep(5m) → if still authorized → capture

# Client cancels before capture
POST /payments/:id/void
  → check status=authorized → send void to Silvergate → status: voided
```

## Changes

### Silvergate — void endpoint

**New handler:** `POST /api/v1/void`
```json
// Request
{ "transaction_id": "txn_abc" }

// Response 200
{ "transaction_id": "txn_abc", "status": "voided" }
```

**Transaction state machine update:**
```
authorized → voided (new)
authorized → capture_pending → captured / capture_failed
```

**Acquirer interface:** add `Void(ctx, txID) (VoidResult, error)` — mock always succeeds (void is rarely declined).

**Webhook:** send `transaction.voided` event to merchant callback.

### Paymanager — capture_delay + void

**Payment entity changes:**
- Add `CaptureDelay time.Duration` field (not persisted — used in service logic)
- Add `CaptureAt *time.Time` field (persisted — when auto-capture should happen)
- State machine: `authorized → voided` transition
- New status: `StatusVoided`

**DB migration:** add `capture_at` column to payments table, add `voided` to status CHECK.

**CreatePaymentRequest update:**
```go
type CreatePaymentRequest struct {
    Amount       int64         `json:"amount" binding:"required,min=1"`
    Currency     string        `json:"currency" binding:"required,len=3"`
    CardToken    string        `json:"card_token" binding:"required"`
    CaptureDelay string        `json:"capture_delay"` // e.g. "5m", "0s" (default)
}
```

**Service logic change in CreatePayment:**
```
if captureDelay == 0:
    mark capture_pending, capture immediately (current behavior)
else:
    save capture_at = now + captureDelay
    start goroutine: sleep(captureDelay) → check status → if authorized → capture
```

**New service method: VoidPayment(ctx, paymentID)**
1. Get payment, check status == authorized
2. Call Silvergate void (sync)
3. Update status → voided

**New handler:** `POST /api/v1/payments/:id/void`

**Gateway Provider:** add `VoidPayment(ctx, VoidRequest) (VoidResult, error)`

**Silvergate client:** add `VoidPayment()` method — POST to `/api/v1/void`

**Webhook handling:** add `transaction.voided` case in ProcessCaptureWebhook.

### Config

- `DEFAULT_CAPTURE_DELAY` env var (default: "0s")

## Implementation Order

1. [ ] Silvergate: void handler, transaction state machine, acquirer Void method, webhook
2. [ ] Silvergate: update e2e test to cover void
3. [ ] Paymanager: migration (capture_at column, voided status)
4. [ ] Paymanager: entity + state machine update (voided status, capture_at)
5. [ ] Paymanager: gateway port + silvergate client (VoidPayment method)
6. [ ] Paymanager: service logic (capture_delay, void, delayed capture goroutine)
7. [ ] Paymanager: void handler + route
8. [ ] Paymanager: webhook consumer (transaction.voided)
9. [ ] Update http/ test files
10. [ ] Config: DEFAULT_CAPTURE_DELAY env var
