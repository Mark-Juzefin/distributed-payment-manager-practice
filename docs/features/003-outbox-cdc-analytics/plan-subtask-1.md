# Plan: Subtask 1 — Transactor refactoring

## Context

Currently each repo owns its transaction via `InTransaction(ctx, func(tx TxOrderRepo) error)`. The callback only provides a repo-scoped view — there's no way to include other participants (like a future event store) in the same transaction.

This refactoring makes services the transaction owner: they get a raw `Executor` and create whatever tx-scoped components they need inside one `BEGIN/COMMIT`.

## Current State

```go
// Service delegates transaction to repo — only TxOrderRepo available inside
err := s.orderRepo.InTransaction(ctx, func(tx TxOrderRepo) error {
    tx.CreateOrder(ctx, event)
    return nil
})
```

## Target State

```go
// Service owns the transaction — can create multiple tx-scoped components
err := s.transactor.InTransaction(ctx, func(tx postgres.Executor) error {
    txRepo := s.txRepoFactory(tx)
    txRepo.CreateOrder(ctx, event)
    return nil
})
```

Behavior is identical — same Serializable isolation, same business logic. The difference is structural: the transaction is no longer locked to one repo.

## Implementation Steps

### Step 1: Add `Transactor` interface to `pkg/postgres/`

**Modify:** `pkg/postgres/postgres.go`

```go
// Transactor provides transaction management.
// *Postgres satisfies this interface automatically.
type Transactor interface {
    InTransaction(ctx context.Context, fn func(tx Executor) error) error
}
```

### Step 2: Export tx-scoped repo constructors

**Modify:** `internal/api/repo/order/pg_order_repo.go`
```go
// NewTxOrderRepo creates a transaction-scoped order repository.
func NewTxOrderRepo(tx postgres.Executor, builder squirrel.StatementBuilderType) order.TxOrderRepo {
    return &repo{db: tx, builder: builder}
}
```

**Modify:** `internal/api/repo/dispute/pg_dispute_repo.go`
```go
// NewTxDisputeRepo creates a transaction-scoped dispute repository.
func NewTxDisputeRepo(tx postgres.Executor, builder squirrel.StatementBuilderType) dispute.TxDisputeRepo {
    return &repo{db: tx, builder: builder}
}
```

### Step 3: Refactor OrderService

**Modify:** `internal/api/domain/order/service.go`

New struct + constructor:
```go
type OrderService struct {
    transactor    postgres.Transactor
    txRepoFactory func(tx postgres.Executor) TxOrderRepo
    orderRepo     OrderRepo       // for reads (GetOrders, GetOrderByID)
    provider      gateway.Provider
    eventSink     EventSink       // unchanged
}
```

Refactor 3 write methods (`ProcessPaymentWebhook`, `UpdateOrderHold`, `CapturePayment`):
- Replace `s.orderRepo.InTransaction(ctx, func(tx TxOrderRepo)` with `s.transactor.InTransaction(ctx, func(tx postgres.Executor)`
- Create tx-scoped repo via `s.txRepoFactory(tx)` inside callback
- Business logic stays the same
- `eventSink` calls stay outside transaction (unchanged)

Read methods (`GetOrderByID`, `GetOrders`, `GetEvents`) — unchanged, use `s.orderRepo` directly.

### Step 4: Refactor DisputeService

**Modify:** `internal/api/domain/dispute/service.go`

Same pattern as OrderService for 3 methods (`ProcessChargeback`, `UpsertEvidence`, `Submit`).

### Step 5: Update DI

**Modify:** `internal/api/app.go`

```go
orderTxRepoFactory := func(tx postgres.Executor) order.TxOrderRepo {
    return order_repo.NewTxOrderRepo(tx, pool.Builder)
}
disputeTxRepoFactory := func(tx postgres.Executor) dispute.TxDisputeRepo {
    return dispute_repo.NewTxDisputeRepo(tx, pool.Builder)
}

orderService := order.NewOrderService(pool, orderTxRepoFactory, orderRepo, silvergateClient, orderEventSink)
disputeService := dispute.NewDisputeService(pool, disputeTxRepoFactory, disputeRepo, silvergateClient, disputeEventSink)
```

Note: `eventStoreFactory` is NOT wired yet — that's Subtask 2.

## Files Summary

| File | Action | What changes |
|------|--------|-------------|
| `pkg/postgres/postgres.go` | MODIFY | Add `Transactor` interface |
| `internal/api/repo/order/pg_order_repo.go` | MODIFY | Add `NewTxOrderRepo` |
| `internal/api/repo/dispute/pg_dispute_repo.go` | MODIFY | Add `NewTxDisputeRepo` |
| `internal/api/domain/order/service.go` | MODIFY | New deps, refactor 3 write methods |
| `internal/api/domain/dispute/service.go` | MODIFY | New deps, refactor 3 write methods |
| `internal/api/app.go` | MODIFY | Wire transactor + factories |

## What stays unchanged

- All database tables
- `eventSink` writes (same position, same behavior)
- API handlers, Kafka consumers
- Read paths, E2E tests

## Verification

1. `make test` — unit tests pass
2. `make e2e-test` — E2E tests pass (behavior identical)
3. `make lint` — no issues
