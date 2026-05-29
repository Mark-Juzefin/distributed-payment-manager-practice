# План: Subtask 2 — `/purchase` endpoint

Реалізація вимог [spec-subtask-2.md](spec-subtask-2.md). План — ordered steps; вимоги, data model, архрішення — у spec.

## Прогрес

- [x] **Step 1: Migration** — `services/silvergate/migrations/<ts>_add_purchase_columns.sql`
  - `ALTER TABLE transactions ADD COLUMN purchase_idempotency_key TEXT`
  - `ALTER TABLE transactions ADD COLUMN product_id UUID REFERENCES products(id) ON DELETE RESTRICT`
  - `CREATE UNIQUE INDEX idx_transactions_purchase_idempotency ON transactions(merchant_id, purchase_idempotency_key) WHERE purchase_idempotency_key IS NOT NULL`
  - Down: drop index → drop columns
  - **NOTE comment у migration file:** "Костиль — generic idempotency_keys table (F-α) is the real fix. Видалити цю колонку коли F-α landed."

- [x] **Step 2: Update `transaction.Transaction` entity** — `internal/transaction/entity.go`
  - Додати поля: `PurchaseIdempotencyKey string`, `ProductID *uuid.UUID`
  - `NewAuthorized` і `NewDeclined` приймають додаткові параметри (або окрема builder method `WithPurchaseContext(key, productID)`)
  - **Recommendation:** окремий setter `(t *Transaction).SetPurchaseContext(key string, productID uuid.UUID)` — чистіший за роздувати конструктори, бо purchase-context optional (legacy `/auth` endpoint не передає)
  - State machine **НЕ чіпаємо** — purchase context не змінює status transitions

- [x] **Step 3: Update `transactionrepo`** — `internal/transaction/transactionrepo/pg.go`
  - `Create`: додати INSERT-колонки `purchase_idempotency_key`, `product_id` (nullable)
  - `GetByID` / `GetByIDForUpdate`: SELECT-додавати нові колонки
  - **New method:** `GetByPurchaseIdempotencyKey(ctx, merchantID, key string) (*Transaction, error)` — для pre-check у purchase.Service. Returns `ErrNotFound` якщо нема row.
  - На INSERT unique-violation `idx_transactions_purchase_idempotency`: bubble error як `ErrPurchaseIdempotencyConflict` (новий sentinel у `transaction/errors.go`); purchase.Service сам re-fetch.

- [x] **Step 4: Split `transaction.Service.Authorize`** — `internal/transaction/service.go`
  - **Виділити** `AuthorizeInTx(ctx context.Context, exec postgres.Executor, req AuthRequest) (*Transaction, error)` — приймає external executor. Виконує acquirer call + repo.Create через `s.txRepo(exec)`.
  - Existing `Authorize(ctx, req)` стає thin wrapper: `s.transactor.InTransaction(...) { s.AuthorizeInTx(ctx, exec, req) }`.
  - Signature `AuthorizeInTx` повертає `*Transaction` (нашлось більше інфи ніж `AuthResponse`) — purchase.Service сам мапить у response.
  - **Verify:** `/auth` HTTP handler не зламано (continues calling `Authorize`, поведінка identical).

- [x] **Step 5: tx-aware factory для `product.Service`** — `internal/product/service.go`
  - Constructor signature changes: `NewService(repo Repo, log *slog.Logger, transactor postgres.Transactor, txRepo func(postgres.Executor) Repo)`.
  - Existing methods (`Create`, `Get`, `List`, `Update`, ...) залишаються — все ще працюють через default `s.repo`.
  - **New helper:** `func (s *Service) MarkPurchasedInTx(ctx context.Context, exec postgres.Executor, merchantID string, id uuid.UUID) error` — використовує `s.txRepo(exec).MarkPurchased(...)`.
  - **Alternative:** замість `MarkPurchasedInTx`, експортуємо `s.RepoWith(exec) Repo` для caller-driven composition. **Не робимо** — encapsulation important.
  - **Update `productrepo.NewPgProductRepo`:** already accepts `postgres.Executor`, ок. Factory function `func(exec) product.Repo { return productrepo.NewPgProductRepo(exec) }` ін'єктується у app.go.
  - **Wire у `app.go`:** новий factory passed at construction.

- [x] **Step 6: New package `internal/purchase/`**

  Files:
  ```
  internal/purchase/
  ├── interfaces.go      — Authorizer, Capturer (consumer-defined)
  ├── service.go         — Service з Purchase method
  ├── errors.go          — ErrIdempotencyConflict, ErrProductArchived (alias), ErrCapturePersistedAuthOnly
  └── service_test.go    — unit tests з hand-rolled fakes
  ```

  **`interfaces.go`:**
  ```go
  type Authorizer interface {
      AuthorizeInTx(ctx context.Context, exec postgres.Executor, req transaction.AuthRequest) (*transaction.Transaction, error)
  }
  type Capturer interface {
      Capture(ctx context.Context, req transaction.CaptureRequest) (transaction.CaptureResponse, error)
  }
  type ProductService interface {
      Get(ctx context.Context, merchantID string, id uuid.UUID) (*product.Product, error)
      MarkPurchasedInTx(ctx context.Context, exec postgres.Executor, merchantID string, id uuid.UUID) error
  }
  type TxLookup interface {
      GetByPurchaseIdempotencyKey(ctx context.Context, merchantID, key string) (*transaction.Transaction, error)
  }
  ```

  **`service.go`:**
  ```go
  type Service struct {
      products   ProductService
      authorizer Authorizer
      capturer   Capturer
      txLookup   TxLookup
      transactor postgres.Transactor
      log        *slog.Logger
  }
  func (s *Service) Purchase(ctx context.Context, req PurchaseRequest) (PurchaseResponse, error)
  ```

  Sequence per [spec §Domain Flow](spec-subtask-2.md#sequence).

- [x] **Step 7: HTTP controller** — `internal/purchase/purchasecontroller/`
  ```
  purchasecontroller/
  ├── purchase.go        — handler
  ├── router.go          — wire route
  └── purchase_test.go   — handler tests з mock service
  ```

  `purchase.go` (handler):
  - Read `X-Merchant-ID` from context (`merchantauth.FromContext`)
  - Read `Idempotency-Key` from `c.GetHeader("Idempotency-Key")` → 400 якщо порожній
  - Parse body — 400 на bind error
  - Call `svc.Purchase(...)` — мапінг errors → status codes per [spec §Error responses](spec-subtask-2.md#error-responses)

- [x] **Step 8: Wire up в `app.go` + `router.go`** — `services/silvergate/`
  - Construct `purchase.Service` з усіма deps
  - `productHandlers.Purchase = purchasecontroller.NewHandler(purchaseSvc)`
  - Add route `/api/v1/purchase` під `merchantauth.Middleware()`

- [x] **Step 9: Unit tests `purchase.Service`** — `internal/purchase/service_test.go`
  - Hand-rolled fakes (mirror Subtask 1 pattern, no gomock):
    - `fakeAuthorizer`
    - `fakeCapturer`
    - `fakeProductService`
    - `fakeTxLookup`
    - `fakeTransactor` (executes callback synchronously з nil executor — fakes ігнорують exec)
  - Test cases per [spec §Domain Flow](spec-subtask-2.md#sequence) + edge cases:
    1. Happy path approved → tx created, MarkPurchased called, Capture invoked, response=capture_pending
    2. Decline → tx created (declined), MarkPurchased NOT called, Capture NOT called, response=declined з reason
    3. Archived product → 422, acquirer NOT called
    4. Cross-merchant product → 404 (через ErrNotFound з productRepo)
    5. Product not found → 404
    6. Pre-check idempotency hit, same hash → cached response, acquirer NOT called
    7. Pre-check idempotency hit, different hash → 409 idempotency_conflict
    8. INSERT race (txRepo повертає ErrPurchaseIdempotencyConflict) → re-fetch → return cached (or 409 якщо mismatch)
    9. Acquirer transport error → 500, tx NOT persisted
    10. Capture failure post-auth → 500 purchase_partially_persisted з tx_id

- [x] **Step 10: Integration tests на UNIQUE constraint** — `internal/transaction/transactionrepo/pg_integration_test.go`
  - Per `.claude/rules/migrations.md`:
    1. INSERT з `(merchant=A, key=K)` → OK
    2. INSERT з `(merchant=A, key=K)` again → `ErrPurchaseIdempotencyConflict`
    3. INSERT з `(merchant=A, key=NULL)` twice → OK (partial index)
    4. INSERT з `(merchant=B, key=K)` → OK (scoped per merchant)
    5. Capture path overwrites `idempotency_key` НЕ torkає `purchase_idempotency_key` — INSERT з K, then Capture з K_cap, then SELECT → `purchase_idempotency_key = K` still
  - Тести `GetByPurchaseIdempotencyKey` для happy + not-found

## Файли для модифікації

| Файл | Зміни |
|------|-------|
| `services/silvergate/migrations/<ts>_add_purchase_columns.sql` | NEW |
| `services/silvergate/internal/transaction/entity.go` | Додати fields + setter |
| `services/silvergate/internal/transaction/errors.go` | Додати `ErrPurchaseIdempotencyConflict` |
| `services/silvergate/internal/transaction/interfaces.go` | Додати `GetByPurchaseIdempotencyKey` до Repo |
| `services/silvergate/internal/transaction/transactionrepo/pg.go` | INSERT/SELECT нові колонки + GetByPurchaseIdempotencyKey |
| `services/silvergate/internal/transaction/transactionrepo/pg_integration_test.go` | Step 10 тести |
| `services/silvergate/internal/transaction/service.go` | Split `Authorize` → `AuthorizeInTx` + wrapper |
| `services/silvergate/internal/product/service.go` | Constructor signature + `MarkPurchasedInTx` |
| `services/silvergate/internal/purchase/interfaces.go` | NEW |
| `services/silvergate/internal/purchase/service.go` | NEW |
| `services/silvergate/internal/purchase/errors.go` | NEW |
| `services/silvergate/internal/purchase/service_test.go` | NEW |
| `services/silvergate/internal/purchase/purchasecontroller/purchase.go` | NEW |
| `services/silvergate/internal/purchase/purchasecontroller/router.go` | NEW |
| `services/silvergate/internal/purchase/purchasecontroller/purchase_test.go` | NEW |
| `services/silvergate/app.go` | DI wiring |
| `services/silvergate/router.go` | Route registration |

## Порядок імплементації

1. **Migration first** (Step 1) — schema ready before code touches it.
2. **Entity + repo** (Steps 2–3) — persistence layer працює без service changes.
3. **Service split** (Step 4) — `AuthorizeInTx` готовий до використання purchase.Service. `/auth` endpoint не зламано.
4. **Product service refactor** (Step 5) — factory pattern dropped in, existing methods unchanged.
5. **Purchase service** (Step 6) — composes готові building blocks.
6. **HTTP** (Steps 7–8) — last mile, минімум логіки.
7. **Tests** (Steps 9–10) — після того як код working end-to-end по manual smoke.

## Open Implementation Decisions

Resolved at write time of each step:

- **Step 2 entity:** окремий setter `SetPurchaseContext` чи extended constructor? Recommendation у Step 2.
- **Step 3 sentinel error:** єдиний `ErrPurchaseIdempotencyConflict` чи розрізняти "conflict" від "race-loss"? Recommendation: один sentinel, service дивиться на cached row.
- **Step 6 request_hash:** як обчислюємо hash request body для conflict detection? SHA256 над JSON canonicalization (sort keys) чи примітивне порівняння полів? Recommendation: примітивне (5 полів, не varying schema).
- **Step 9 fakeTransactor:** як підтримуємо нескінченність вкладень? Recommendation: один-рівнева транзакція, fake виконує callback з `nil postgres.Executor`, fakes на repos ігнорують exec параметр.

## Known Limitations (also у spec)

Cross-link: [spec §Known Limitations / Future Work](spec-subtask-2.md#known-limitations--future-work) описує F-α (generic idempotency table), F-β (intent-record pattern), F-γ (compensating Void). Кожне — потенційна окрема feature після завершення 008.
