# Spec: Subtask 2 — `/purchase` endpoint у Silvergate

> Цей документ — **вимоги** (Jira-style ticket), не план імплементації.
> План з кодом, файлами і порядком кроків — [plan-subtask-2.md](plan-subtask-2.md).

## Мета

Додати високорівневий endpoint `POST /api/v1/purchase`, який композує існуючі `transaction.Service.Authorize` + `transaction.Service.Capture` навколо `product.Service.Get`. Caller передає `{order_id, product_id, card_token}` + `Idempotency-Key` header — сервер сам тягне ціну/валюту з product, авторизує транзакцію через acquirer, capture'ить asynchronously.

Це перша feature де ми вправляємось на shared DB transaction (зв'язування двох доменних writes — transactions + products — атомарно) і на caller-driven idempotency через `Idempotency-Key` header.

## In Scope

- HTTP endpoint `POST /api/v1/purchase` (під existing `merchantauth.Middleware`).
- Domain service `purchase.Service` у новому пакеті `services/silvergate/internal/purchase/`.
- Композиція: validate product → acquirer.Authorize → persist transaction → MarkPurchased → trigger Capture.
- Shared DB transaction обгортає `acquirer.Authorize + INSERT transaction + UPDATE products.first_purchased_at`.
- Caller-provided idempotency через header `Idempotency-Key`. Pre-check + INSERT-race backstop через новий UNIQUE constraint.
- Consumer-defined interfaces (`purchase.Authorizer`, `purchase.Capturer`) для тестабельності.
- Migration: додати колонку `transactions.purchase_idempotency_key` + partial UNIQUE index.
- Unit tests на `purchase.Service` (pure mocks), integration tests на UNIQUE constraint у `transactionrepo/`.

## Out of Scope (Subtask 2)

- **`idempotency_keys` generic table** — see [Known Limitations §F-α](#known-limitations--future-work).
- **Intent-record + reconciliation worker** для acquirer call безпечно поза DB tx — see [§F-β](#known-limitations--future-work).
- **Compensating Void (saga)** при Capture failure — see [§F-γ](#known-limitations--future-work).
- **Quantity / multi-product cart / discounts** — single product per purchase у v1.
- **Caller-supplied amount/currency у body** — derived з product server-side; defense проти client price tampering.
- **Refund flow інтеграція з products** — refund поки що транзакція-only.
- **e2e tests** — deferred consistent з [Subtask 1 test strategy](README.md).
- **402 для decline** — decline = успішний business outcome, response 200 з `status: "declined"`.

## API Surface

### Request

```
POST /api/v1/purchase
X-Merchant-ID: merchant-123
Idempotency-Key: <client-provided-uuid-or-string>
Content-Type: application/json

{
  "order_id": "ord_42",
  "product_id": "550e8400-e29b-41d4-a716-446655440000",
  "card_token": "tok_visa_1111"
}
```

**Headers:**
- `X-Merchant-ID` — required, через `merchantauth.Middleware`. Відсутній → 401.
- `Idempotency-Key` — required. Caller-controlled string (рекомендовано UUID). Відсутній → 400 `missing_idempotency_key`.

**Body fields (всі required):**
- `order_id` — caller's order reference. Зберігається у `transactions.order_ref`.
- `product_id` — UUID існуючого product (того ж merchant).
- `card_token` — opaque token, передається у acquirer.

**Поля які НЕ передаються (defense-in-depth):**
- `amount` — береться з `product.Price`.
- `currency` — береться з `product.Currency`.
- `merchant_id` — береться з header context.

### Response — approved (200 OK)

```json
{
  "transaction_id": "9f86d081-884c-4d65-9b67-1b69adfb12ad",
  "product_id": "550e8400-e29b-41d4-a716-446655440000",
  "order_id": "ord_42",
  "status": "capture_pending",
  "amount": 1999,
  "currency": "USD"
}
```

### Response — declined (200 OK)

```json
{
  "transaction_id": "9f86d081-884c-4d65-9b67-1b69adfb12ad",
  "product_id": "550e8400-e29b-41d4-a716-446655440000",
  "order_id": "ord_42",
  "status": "declined",
  "decline_reason": "insufficient_funds"
}
```

Decline — успішний HTTP response. Acquirer повернув `Approved=false`, ми зафіксували tx з `status=declined`, MarkPurchased НЕ викликали, Capture НЕ викликали.

### Error responses

| Status | Code | Coнтекст |
|--------|------|----------|
| 400 | `invalid_request` | Malformed JSON, missing required body fields |
| 400 | `missing_idempotency_key` | Header `Idempotency-Key` відсутній |
| 401 | (from middleware) | Header `X-Merchant-ID` відсутній |
| 404 | `product_not_found` | Product не існує OR належить іншому merchant |
| 409 | `idempotency_conflict` | Same `Idempotency-Key` був використаний з іншим body |
| 422 | `product_archived` | Product існує але `status=archived` |
| 500 | `internal_error` | DB error, acquirer transport error, unexpected failure |
| 500 | `purchase_partially_persisted` | Authorize succeeded (money authorized) але Capture failed. Response містить `transaction_id` для manual recovery. |

## Domain Flow

### Sequence

```
1. HTTP layer:
   - Parse body
   - Extract X-Merchant-ID from context (middleware)
   - Extract Idempotency-Key header
   - Validate required fields

2. purchase.Service.Purchase(ctx, req):
   a. Pre-check idempotency (outside any DB tx):
      - SELECT tx WHERE merchant_id = X AND purchase_idempotency_key = K
      - If found:
        - Compare request_hash to stored (наразі ми зберігаємо ТУПО самі поля у tx: order_ref, amount, currency, card_token, product_id)
        - Mismatch → 409 idempotency_conflict
        - Match → return cached response (skip acquirer)
      - If not found: продовжуємо

   b. Open DB tx (RepeatableRead):
      - productRepo.GetByID(merchantID, productID)
        - Not found / cross-merchant → ErrNotFound → rollback, return 404
        - Archived → ErrProductArchived → rollback, return 422
      - acquirer.Authorize(p.Price, p.Currency, req.CardToken)
        - Transport error → rollback, return 500
      - Construct transaction:
        - Approved → NewAuthorized(merchantID, orderID, p.Price, p.Currency, cardToken)
        - Declined → NewDeclined(merchantID, orderID, p.Price, p.Currency, cardToken, reason)
        - Set tx.PurchaseIdempotencyKey = K
        - Set tx.ProductID = productID  // нове поле, see Data Model
      - txRepo.Create(tx)
        - Unique violation на (merchant_id, purchase_idempotency_key) → re-fetch existing → compare → return cached or 409
      - If Approved:
        - productRepo.MarkPurchased(merchantID, productID)  // idempotent
      - Commit
        - DB error → rollback, return 500 (acquirer уже approved → see §F-β)

   c. If Approved (outside purchase tx):
      - txSvc.Capture(ctx, CaptureRequest{
          TransactionID: tx.ID,
          Amount: tx.Amount,
          IdempotencyKey: tx.ID.String() + "-cap",
        })
      - Capture failure → log + return 500 purchase_partially_persisted з tx_id у body

   d. Return response:
      - Approved → status=capture_pending
      - Declined → status=declined з decline_reason
```

### Validation rules

| Rule | Behavior | Reason |
|------|----------|--------|
| Product not found | 404 `product_not_found` | Standard |
| Product belongs to other merchant | 404 `product_not_found` | Defense-in-depth, не leakaємо existence |
| Product archived (`status='archived'`) | 422 `product_archived` | Cannot buy what's not for sale |
| Product locked (`first_purchased_at IS NOT NULL`) | **OK, allowed** | Lock freezes editing, не buying |
| Amount/currency у body | Ignored | Source of truth = product |
| Idempotency-Key reuse, same body | 200 cached response | Standard idempotency |
| Idempotency-Key reuse, different body | 409 `idempotency_conflict` | Stripe-style guard |

### Data Model — additions

**`transactions` table:**

| Колонка | Тип | Обмеження | Notes |
|---------|-----|-----------|-------|
| `purchase_idempotency_key` | TEXT | NULL OK | Set by `/purchase`, never overwritten |
| `product_id` | UUID | NULL OK, FK `products(id)` ON DELETE RESTRICT | Set by `/purchase`. NULL для tx створених через legacy `/auth` endpoint |

**New index:**
- `idx_transactions_purchase_idempotency` — `UNIQUE (merchant_id, purchase_idempotency_key) WHERE purchase_idempotency_key IS NOT NULL`

**Existing `idempotency_key` column НЕ чіпаємо** — залишаємо як capture-action key. Capture continue overwrite-ить його (pre-existing bug, [§F-α](#known-limitations--future-work)).

## Architectural Decisions

### Why composition lives у new `internal/purchase/` package
- Canonical layout pattern (`internal/<domain>/<domainXxx>/`) — mirror `product/`, `transaction/`.
- `transaction.Service` не повинен знати про products — порушує isolation.
- Roadmap натякає на subscriptions/ledger — composition layer ростиме.
- Альтернатива (composition у controller) ховає business rules у HTTP layer.

### Why shared DB transaction wraps acquirer call (naive)
- Спрощує atomicity reasoning для learning project.
- **Trade-off:** acquirer.Authorize всередині tx = long DB lock + risk lost-result на crash. Реальний fix — intent-record pattern, [§F-β](#known-limitations--future-work).
- Альтернатива (atomic write-after-acquirer) потребує acquirer idempotency + reconciliation. Окрема feature.

### Why split `transaction.Service.Authorize` через `AuthorizeInTx`
- Existing `Authorize` створює свою repo connection. Не можна композувати у shared tx.
- `AuthorizeInTx(ctx, exec, req)` приймає external executor → працює всередині purchase tx.
- Old `Authorize` стає wrapper навколо `AuthorizeInTx` з default repo (для `/auth` endpoint).

### Why consumer-defined interfaces (`purchase.Authorizer`, `purchase.Capturer`)
- `transaction.Service` зараз — concrete struct. Витягнути interface у самому `transaction` пакеті = blast radius на всі handlers.
- Consumer-defined interfaces у `internal/purchase/interfaces.go` — Go idiomatic. Lokal до того хто потребує мокаble dependency.

### Why caller-provided Idempotency-Key (не server-derived)
- HTTP industry standard (Stripe, Adyen).
- Caller контролює retry semantics.
- Server-derived з `(merchant, order_id, product_id)` ламає сценарії multi-product orders.

### Why новий column `purchase_idempotency_key` (костиль)
- Existing `transactions.idempotency_key` overwrite-ить при Capture (`MarkCapturePending` mutates field).
- Окрема колонка для purchase ізолює два idempotency lanes.
- **Це не правильне рішення.** Real fix — generic `idempotency_keys` table, [§F-α](#known-limitations--future-work).

### Why Capture викликаємо ПОЗА purchase tx
- Capture має власну tx з RepeatableRead isolation. Mixing isolation levels у одному tx ускладнює reasoning.
- Race window (повторний request на ту ж tx) — теоретичний: tx тільки створена, ніхто ще не знає її ID.

### Why MarkPurchased викликаємо при Authorize approved (не при Capture settled)
- Між Authorize і settle merchant міг би edit price → settle refер до stale snapshot.
- Lock одразу як гроші reserved.
- При decline — MarkPurchased НЕ викликаємо (tx у declined state, нічого не fix'ено).

## Known Limitations / Future Work

Зафіксовано тут щоб не загубилось. Кожне — потенційна окрема feature.

### F-α: Generic `idempotency_keys` table

**Поточний костиль:** колонка `transactions.purchase_idempotency_key`. Окремо від існуючого `idempotency_key` (capture-action key, який Capture overwrite-ить — pre-existing bug).

**Правильний pattern (Stripe-style):**
```sql
CREATE TABLE idempotency_keys (
    merchant_id   TEXT NOT NULL,
    key           TEXT NOT NULL,
    endpoint      TEXT NOT NULL,         -- 'purchase', 'capture', 'refund', ...
    request_hash  TEXT NOT NULL,
    response_status INT NOT NULL,
    response_body JSONB NOT NULL,
    created_at    TIMESTAMPTZ NOT NULL DEFAULT now(),
    expires_at    TIMESTAMPTZ NOT NULL,  -- 24h TTL
    PRIMARY KEY (merchant_id, key, endpoint)
);
```

Generic helper / middleware:
1. Lookup by `(merchant, key, endpoint)`.
2. If hit + hash match → return cached response.
3. If hit + hash mismatch → 409.
4. If miss → INSERT placeholder → execute work → UPDATE row з response.

Усі endpoints (purchase, capture, refund, future ones) використовують один механізм. Колонки `idempotency_key` і `purchase_idempotency_key` з `transactions` видаляються.

**Виправляє також pre-existing Capture overwrite bug.**

### F-β: Intent-record pattern для acquirer safety

**Поточна проблема:** `acquirer.Authorize` викликається всередині DB tx. Якщо commit fails після bank approval → bank held money, ми не маємо запису. Lost-result.

**Правильний pattern:**
```
Phase 1 (DB tx): INSERT transaction (status='authorizing', id=tx_id, idempotency_key=K)
Phase 2 (no tx): result := acquirer.Authorize(idempotency_key=tx_id)
Phase 3 (DB tx): UPDATE transaction SET status = approved ? 'authorized' : 'declined'
Phase 4 (worker): reconciliation для stuck 'authorizing' — query bank, sync state
```

Потребує:
- New status `authorizing` у state machine.
- Acquirer повинен дедуплити по idempotency_key (real banks роблять, mock — потрібно extend).
- Background reconciliation worker.

### F-γ: Compensating Void (saga)

**Поточна проблема:** Authorize succeeded, Capture failed → tx залишається у `authorized` state. Caller отримує `purchase_partially_persisted` 500 з tx_id і має manually вибрати void або retry capture.

**Правильний pattern:** при Capture fail синхронно (або через worker) викликаємо `transaction.Service.Void` щоб release hold. Якщо Void теж fail → persistent retry queue з alerting.

Потребує:
- Outbox table для compensating actions.
- Worker що retries Void з backoff.
- Idempotency для Void calls на acquirer.

## Migration Test Coverage

Per `.claude/rules/migrations.md`, нова UNIQUE constraint вимагає integration test:

```go
// services/silvergate/internal/transaction/transactionrepo/pg_integration_test.go
func TestCreateTx_PurchaseIdempotencyConstraint(t *testing.T) {
    // 1. INSERT з (merchant=A, key=K) → OK
    // 2. INSERT з (merchant=A, key=K) again → unique violation
    // 3. INSERT з (merchant=A, key=NULL) twice → OK (partial index)
    // 4. INSERT з (merchant=B, key=K) → OK (scoped)
    // 5. Capture overwrite на `idempotency_key` НЕ torkає `purchase_idempotency_key`
}
```
