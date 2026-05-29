# Transactions & Consistency

How this codebase runs multi-step DB work atomically, composes several services
inside one transaction, and keeps state correct under concurrency — without
leaking `pgx.Tx` into the domain layer.

Canonical source: `pkg/postgres/postgres.go`,
`services/silvergate/internal/transaction/` (service, repo, entity),
`services/silvergate/internal/purchase/service.go`.

---

## 1. `Transactor` + `Executor` abstraction

Two tiny interfaces decouple repositories from *how* they run. `Executor` is the
common subset of `*pgxpool.Pool` and `pgx.Tx`; `Transactor` opens a transaction.

```go
// pkg/postgres/postgres.go
type Executor interface {
    Query(ctx context.Context, sql string, args ...any) (pgx.Rows, error)
    QueryRow(ctx context.Context, sql string, args ...any) pgx.Row
    Exec(ctx context.Context, sql string, args ...any) (pgconn.CommandTag, error)
}

type Transactor interface {
    InTransaction(ctx context.Context, isoLevel pgx.TxIsoLevel, fn func(tx Executor) error) error
}
```

A repository holds a `postgres.Executor`, never a concrete pool or tx:

```go
// services/silvergate/internal/transaction/transactionrepo/pg.go
type PgTransactionRepo struct{ db postgres.Executor }
func NewPgTransactionRepo(db postgres.Executor) *PgTransactionRepo { return &PgTransactionRepo{db: db} }
```

**Why:** the same repo code works on the pool (autocommit) and inside a tx. No
`txRepo`/`nonTxRepo` duplication, no `*sql.Tx` threaded through signatures.
`*Postgres` satisfies `Transactor` automatically, so the domain depends only on
the interface.

Refs: `pkg/postgres/postgres.go:79` (Executor), `:87` (Transactor), `:91` (impl).

---

## 2. `InTransaction` with an **explicit** isolation level

The commit/rollback boilerplate lives in exactly one place. Callers pass the
isolation level deliberately — it is never implicit.

```go
// pkg/postgres/postgres.go
func (p *Postgres) InTransaction(ctx context.Context, isoLevel pgx.TxIsoLevel, fn func(tx Executor) error) (err error) {
    tx, err := p.Pool.BeginTx(ctx, pgx.TxOptions{IsoLevel: isoLevel})
    if err != nil { return fmt.Errorf("start transaction: %w", err) }
    defer func() {
        if err != nil { _ = tx.Rollback(ctx) } // rollback iff fn or commit failed
    }()
    if err = fn(tx); err != nil { return err }
    if err = tx.Commit(ctx); err != nil { return fmt.Errorf("failed to commit transaction: %w", err) }
    return nil
}
```

**Why the named return + `defer`:** rollback is driven by the *final* value of
`err`, so any early return from `fn` rolls back; a successful commit leaves
`err == nil` and skips rollback. One implementation, impossible to forget.

Ref: `pkg/postgres/postgres.go:91`.

---

## 3. The repo-factory pattern

A service that needs transactions holds **two** collaborators: a `Transactor`
and a factory that binds a repo to a given `Executor`.

```go
// services/silvergate/internal/transaction/service.go
type Service struct {
    repo       Repo                              // pool-bound, for non-tx calls
    transactor postgres.Transactor               // opens transactions
    txRepo     func(postgres.Executor) Repo       // binds a repo to the tx
    // ...acquirer, webhooks, log
}
```

Wired by hand in `app.go`:

```go
// services/silvergate/app.go
txRepo := transactionrepo.NewPgTransactionRepo(pg.Pool)             // autocommit repo
txRepoFactory := func(tx postgres.Executor) transaction.Repo {       // tx-bound repo factory
    return transactionrepo.NewPgTransactionRepo(tx)
}
svc := transaction.NewService(txRepo, acq, webhookSender, log, pg, txRepoFactory)
```

Inside a transaction the service rebinds the repo to the live `Executor`:

```go
err := s.transactor.InTransaction(ctx, pgx.RepeatableRead, func(dbTx postgres.Executor) error {
    txRepo := s.txRepo(dbTx)                 // same repo type, now tx-scoped
    tx, err := txRepo.GetByID(ctx, id)
    // ...mutate, then txRepo.UpdateStatus(ctx, tx)
    return err
})
```

**Why:** the alternative — passing `pgx.Tx` into every repo method — pollutes the
domain contract with infra types. The factory keeps `Repo` clean and lets one
service mix autocommit calls (`s.repo`) and transactional calls (`s.txRepo(tx)`).

Refs: `services/silvergate/internal/transaction/service.go:16`, `services/silvergate/app.go:56`.

---

## 4. The `…InTx` method pair (compose, don't nest transactions)

Each operation comes in two flavours: a transaction-owning entry point and an
`…InTx` variant that runs against a caller-supplied repo. This lets the same
logic be a standalone call *or* a step inside a larger transaction — without
nested transactions.

```go
// Standalone: owns no tx, runs on the pool repo.
func (s *Service) Authorize(ctx context.Context, req AuthRequest) (AuthResponse, error) {
    tx, err := s.AuthorizeInTx(ctx, s.repo, req)
    // ...map to response
}

// Composable: persists via the repo the caller passes (possibly tx-bound).
func (s *Service) AuthorizeInTx(ctx context.Context, repo Repo, req AuthRequest) (*Transaction, error) {
    result, err := s.acq.Authorize(ctx, req.Amount, req.Currency, req.CardToken) // external call
    // ...build Transaction, then:
    if err := repo.Create(ctx, tx); err != nil { return nil, fmt.Errorf("save transaction: %w", err) }
    return tx, nil
}
```

**Why:** Go has no nested transactions on a single connection. Exposing an
`…InTx(repo, …)` seam is how callers opt the operation into *their* transaction.
**Trade-off:** the external acquirer call sits inside `AuthorizeInTx`, so when it
is composed into a tx, that tx is held open across a network call. Acceptable
here (short call, low volume); for hot paths you would authorize first, then open
a short tx only for the DB writes.

Ref: `services/silvergate/internal/transaction/service.go:61`.

---

## 5. Composing **multiple services** in one transaction

`/purchase` authorizes a card *and* marks a product purchased atomically. Both
services expose `…InTx` methods that accept the same `Executor`, so one
`InTransaction` spans both.

```go
// services/silvergate/internal/purchase/service.go
err = s.transactor.InTransaction(ctx, pgx.ReadCommitted, func(exec postgres.Executor) error {
    tx, authErr := s.authorizer.AuthorizeInTx(ctx, s.txRepo(exec), transaction.AuthRequest{...})
    if authErr != nil { return authErr }                 // returning error → whole tx rolls back
    if tx.Status == transaction.StatusAuthorized {
        if err := s.products.MarkPurchasedInTx(ctx, exec, req.MerchantID, req.ProductID); err != nil {
            return fmt.Errorf("mark product purchased: %w", err)
        }
    }
    return nil
})
// capture happens AFTER commit — see pattern 10
```

**Why:** the orchestrating service (`purchase`) owns the transaction boundary;
the participating services (`transaction`, `product`) stay unaware of each other.
Any error short-circuits to a single rollback covering both writes.

Ref: `services/silvergate/internal/purchase/service.go:91`.

---

## 6. Pessimistic lock: `SELECT … FOR UPDATE`

When a row must not change between read and write (e.g. void must block a
concurrent capture), lock it. The repo exposes a dedicated method; the query is
just the normal select with a suffix.

```go
// transactionrepo/pg.go
func (r *PgTransactionRepo) GetByIDForUpdate(ctx context.Context, id uuid.UUID) (*transaction.Transaction, error) {
    query, args, _ := psql.Select(cols...).From("transactions").
        Where(sq.Eq{"id": id}).Suffix("FOR UPDATE").ToSql()
    return scanTransaction(r.db.QueryRow(ctx, query, args...))
}
```

```go
// service.go — Void: ReadCommitted is enough because the row is locked
err := s.transactor.InTransaction(ctx, pgx.ReadCommitted, func(dbTx postgres.Executor) error {
    tx, _ := s.txRepo(dbTx).GetByIDForUpdate(ctx, txID)   // row locked until commit
    if err := tx.MarkVoided(); err != nil { return err }
    // acquirer.Void while holding the lock → no concurrent capture can proceed
    // ...UpdateStatus
})
```

**Why / trade-off:** `FOR UPDATE` is the simplest correct answer for "read,
decide, write, no one else touches this row". Cost: it holds a row lock for the
duration of the tx (here, across the bank `Void` call). For high-contention rows
prefer the CAS pattern below.

Ref: `transactionrepo/pg.go:104`, `service.go:280` (Void).

---

## 7. Optimistic concurrency: compare-and-swap

For async state moves where you don't want to hold a lock, do a conditional
`UPDATE` and treat "0 rows affected" as a lost race.

```go
// transactionrepo/pg.go
func (r *PgTransactionRepo) CompareAndUpdateStatus(ctx context.Context, id uuid.UUID, expected, next Status) error {
    res, err := r.db.Exec(ctx, `UPDATE transactions SET status=$1, updated_at=now()
                                 WHERE id=$2 AND status=$3`, next, id, expected)
    if err != nil { return err }
    if res.RowsAffected() == 0 { return transaction.ErrStatusChanged } // someone else moved it
    return nil
}
```

```go
// service.go — settleAsync (runs in a goroutine, no surrounding tx)
if err := s.repo.CompareAndUpdateStatus(ctx, tx.ID, StatusCapturePending, nextStatus); err != nil {
    // ErrStatusChanged → the transaction was voided/changed meanwhile; don't clobber it
}
```

**Why:** no lock, no transaction needed — the `WHERE status = expected` clause is
the guard. Ideal for fire-and-forget background updates where the row may have
moved on. **Trade-off:** caller must handle `ErrStatusChanged` (usually: log and
drop, since a newer state already won).

Ref: `transactionrepo/pg.go:157`, `service.go:329` (settleAsync).

---

## 8. Reserve / release (two-phase value mutation)

Refund reserves the amount *inside* the tx (so concurrent refunds can't
oversell), calls the bank asynchronously, and releases the reservation if the
bank rejects.

```go
// service.go — Refund: reserve within the tx
err := s.transactor.InTransaction(ctx, pgx.RepeatableRead, func(dbTx postgres.Executor) error {
    txRepo := s.txRepo(dbTx)
    tx, _ := txRepo.GetByID(ctx, req.TransactionID)
    if tx.Status != StatusCaptured && tx.Status != StatusPartiallyRefunded { return ErrNotRefundable }
    if req.Amount > tx.Amount-tx.RefundedAmount { return ErrRefundExceedsAmount } // write-skew guard
    tx.RefundedAmount += req.Amount                       // reserve
    // ...set status, txRepo.UpdateRefund(tx); txRepo.CreateRefund(pendingRefund)
    return nil
})
// ...later, in refundAsync(), if the acquirer rejects:
s.repo.ReleaseRefundAmount(ctx, tx.ID, refund.Amount)     // give it back, with retry
```

```go
// transactionrepo/pg.go — release is a self-referential UPDATE (no read needed)
`UPDATE transactions
   SET refunded_amount = refunded_amount - $1,
       status = CASE WHEN refunded_amount - $1 <= 0 THEN 'captured' ELSE 'partially_refunded' END,
       updated_at = now()
 WHERE id = $2`
```

**Why:** reserving under `RepeatableRead` inside the tx prevents two valid-alone
partial refunds from together exceeding the captured amount (write skew). Release
is an in-place arithmetic `UPDATE`, so it needs no prior read and is safe to
retry.

Refs: `service.go:170` (Refund), `:233` (refundAsync), `transactionrepo/pg.go:237`.

---

## 9. Idempotency keys (pre-check + unique constraint + race resolve)

Three layers defend against duplicate `/purchase` requests:

1. **Pre-check** — look up by `(merchant_id, idempotency_key)`; on hit, replay
   the cached response (and verify it's the *same* request, else `409`).
2. **Unique constraint** — DB index is the real guard against the race where two
   requests both pass the pre-check.
3. **Race resolve** — on the unique-violation error, re-fetch and return the
   winner's result.

```go
// purchase/service.go
func (s *Service) Purchase(ctx context.Context, req Request) (Response, error) {
    if cached, ok, err := s.checkIdempotency(ctx, req); err != nil {       // 1. pre-check
        return Response{}, err
    } else if ok {
        return cached, nil
    }
    // ...authorize+persist in tx...
    if err != nil {
        if errors.Is(err, transaction.ErrPurchaseIdempotencyConflict) {     // 3. lost the race
            return s.resolveRace(ctx, req)                                   //    re-fetch winner
        }
        return Response{}, err
    }
}
```

The repo maps Postgres error `23505` to a domain sentinel **by constraint name**,
so different unique indexes produce different domain errors:

```go
// transactionrepo/pg.go
func mapUniqueViolation(err error) error {
    var pgErr *pgconn.PgError
    if !errors.As(err, &pgErr) || pgErr.Code != "23505" { return nil }
    switch pgErr.ConstraintName {
    case "idx_transactions_purchase_idempotency": return transaction.ErrPurchaseIdempotencyConflict
    default:                                       return transaction.ErrDuplicateIdempotency
    }
}
```

**Why:** the pre-check is the fast path; the unique index is the *correctness*
guarantee (pre-check alone has a TOCTOU race). Mapping `23505` by constraint name
turns infra errors into meaningful domain decisions.

Refs: `purchase/service.go:75` (Purchase), `:148` (checkIdempotency), `:166` (resolveRace), `transactionrepo/pg.go:259` (mapUniqueViolation).

---

## 10. Side effects **outside** the transaction

Anything non-DB (calling the bank to settle, sending a webhook) happens *after*
commit, usually in a goroutine — never inside the tx.

```go
// purchase/service.go — capture runs only after the tx committed
if tx.Status == transaction.StatusAuthorized {
    if _, capErr := s.capturer.Capture(ctx, ...); capErr != nil {
        return partialResponse, ErrCapturePartiallyApplied   // explicit partial-failure signal
    }
}
```

```go
// service.go — webhook fired in a goroutine after commit
go s.settleAsync(tx, req.Amount)
```

**Why:** holding a tx open across external calls bloats lock duration and risks
committing work that depends on a side effect that later fails. The cost is an
intermediate state visible to clients (`capture_pending`, or
`ErrCapturePartiallyApplied`) — which the API surfaces honestly rather than
hiding behind a long transaction.

Ref: `purchase/service.go:120`, `service.go:149`.

---

## Isolation-level cheat-sheet (as used here)

| Situation | Choice | Where |
|-----------|--------|-------|
| Read-modify-write on a row that must not change concurrently | `ReadCommitted` + `SELECT … FOR UPDATE` | `Void` |
| Multi-row invariant / write-skew risk (refund vs captured total) | `RepeatableRead` | `Refund`, `Capture` |
| Fire-and-forget async status move, no lock wanted | no tx + compare-and-swap | `settleAsync` |
| Multi-service atomic write, no single hot row | `ReadCommitted` (composition) | `/purchase` |

Rule of thumb in this repo: **lock the row (`FOR UPDATE`) when you can name the
row; raise isolation (`RepeatableRead`) when the invariant spans rows; use CAS
when you hold no transaction at all.**

## Related

- Domain errors that these flows return → [ddd-structure.md](ddd-structure.md) §4
- Outbox writes in the *same* tx as business data → [messaging.md](messaging.md) §3
- Testing tx flows with a fake `Transactor` → [infra-testing.md](infra-testing.md) §4
