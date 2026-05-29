# Code Structure & DDD

How the codebase is organised: a Go-workspace monorepo of isolated service
modules, each split into per-domain `internal/` packages with guarded
aggregates, sentinel errors, and ports & adapters.

Canonical source: `go.work`, `services/silvergate/internal/transaction/`
(the cleanest example), `services/silvergate/app.go`.

---

## 1. Go-workspace monorepo (compiler-enforced isolation)

`go.work` ties together one module per service plus a shared `pkg/` module. Each
service has its own `go.mod` and **cannot import another service** — the Go
compiler enforces the boundary.

```
go.work
pkg/                      # shared library module (TestTaskJustPay/pkg)
services/
  silvergate/             # own go.mod (TestTaskJustPay/services/silvergate)
  paymanager/             # own go.mod
  ingest/  cdc/  analytics/
```

**Why:** the isolation of microservices (no sneaky cross-service imports) with
the ergonomics of a monorepo (one checkout, shared `pkg/`, atomic refactors).
**Trade-off:** `go.work` is great for local dev; for CI/release you typically
build each module independently so the workspace file doesn't mask a missing
`require`.

---

## 2. Per-domain `internal/` package layout

Every bounded context is one package under `internal/`, with a fixed file
breakdown. Transports and persistence are sub-packages.

```
internal/transaction/
  entity.go         # aggregate, value objects, status enum, state-machine rules
  interfaces.go     # ports the domain needs: Repo, WebhookSender, ...
  service.go        # use-case orchestration (the only "smart" file)
  errors.go         # domain-error sentinels
  refund.go         # a secondary entity in the same context
  transactionrepo/  # PgTransactionRepo implements interfaces.go:Repo
  transactioncontroller/  # one file per transport: auth.go, capture.go, refund.go, void.go
```

**Why:** opening one folder shows the whole context — its data, its contracts,
its rules — with no guessing where logic lives. `internal/` makes those types
unimportable from other services even within the workspace.

Ref: browse `services/silvergate/internal/transaction/`.

---

## 3. Aggregates with **guarded** transitions (state machine in the domain)

State changes go through methods on the aggregate that validate the transition;
illegal moves return a domain error instead of mutating. Construction goes
through named constructors, not bare struct literals.

```go
// entity.go
func NewAuthorized(merchantID, orderRef string, amount int64, currency, cardToken string) *Transaction { ... }

func (t *Transaction) MarkCapturePending(idempotencyKey string) error {
    if t.Status != StatusAuthorized { return ErrInvalidTransition }   // guard
    t.Status = StatusCapturePending
    t.IdempotencyKey = idempotencyKey
    t.UpdatedAt = time.Now().UTC()
    return nil
}

// the legal graph, declared once:
var validTransitions = map[Status][]Status{
    StatusAuthorized:     {StatusCapturePending, StatusVoided},
    StatusCapturePending: {StatusCaptured, StatusCaptureFailed},
    StatusCaptured:       {StatusPartiallyRefunded, StatusRefunded},
    // ...
}
func (s Status) CanTransitionTo(target Status) bool { ... }
```

**Why:** the service layer can't accidentally drive an order into an impossible
state — the aggregate owns its invariants. The `validTransitions` map documents
the lifecycle in one place and is unit-testable in isolation.

Ref: `services/silvergate/internal/transaction/entity.go:43` (constructors), `:79` (guards), `:116` (transition map).

---

## 4. Domain errors as sentinels

Each context declares its errors in `errors.go` as package-level `errors.New`
values. Services return them; repositories translate infra errors into them
(see [transactions.md](transactions.md) §9); controllers map them to HTTP codes.

```go
// errors.go
var (
    ErrNotFound                    = errors.New("transaction not found")
    ErrRefundExceedsAmount         = errors.New("refund amount exceeds remaining balance")
    ErrNotRefundable               = errors.New("transaction is not in a refundable state")
    ErrStatusChanged               = errors.New("transaction status was changed by another operation")
    ErrPurchaseIdempotencyConflict = errors.New("purchase idempotency key already used")
)
```

**Why:** callers branch with `errors.Is`, not string matching. The domain error
is the single contract between layers; only the controller knows about HTTP, only
the repo knows about `pgconn.PgError`.

Ref: `services/silvergate/internal/transaction/errors.go`.

---

## 5. Ports & adapters (dependency inversion)

The domain declares the interfaces it needs (`interfaces.go`); infrastructure
implements them. The domain imports no infra package.

```go
// interfaces.go — the domain's outbound ports
type Repo interface {
    Create(ctx context.Context, tx *Transaction) error
    GetByIDForUpdate(ctx context.Context, id uuid.UUID) (*Transaction, error)
    CompareAndUpdateStatus(ctx context.Context, id uuid.UUID, expected, next Status) error
    // ...
}
type WebhookSender interface {
    SendCaptureResult(ctx context.Context, tx *Transaction) error
}

// acquirer/port.go — a port for the external bank
type Acquirer interface {
    Authorize(ctx context.Context, amount int64, currency, cardToken string) (AuthResult, error)
    Settle(ctx context.Context, txID string, amount int64) (SettleResult, error)
    // ...
}
```

Adapters: `transactionrepo.PgTransactionRepo` implements `Repo`,
`acquirer.MockAcquirer` implements `Acquirer`, `webhooksender.Sender` implements
`WebhookSender`.

**Why:** swap Postgres for an in-memory repo, or the mock bank for a real HTTP
client, with zero domain changes — and unit tests inject fakes (see
[infra-testing.md](infra-testing.md) §4).

Refs: `services/silvergate/internal/transaction/interfaces.go`, `services/silvergate/internal/acquirer/port.go`.

---

## 6. "Complex opt-in": the heavy implementation is a swappable adapter

Because dependencies are interfaces, the *expensive* implementation is always
optional. The same domain runs simple or complex depending on which adapter is
wired in `app.go`.

| Port | Simple adapter | Complex adapter |
|------|----------------|-----------------|
| `webhook.Processor` (ingest) | direct HTTP forward | Kafka publish / inbox table |
| `Acquirer` | in-process mock | real PSP HTTP client |
| `Executor` | `*pgxpool.Pool` (single DB) | tx / replica-routed pool |
| event delivery | poll table | outbox → CDC → Kafka |

**Rule when porting a pattern:** depend on the interface; keep the heavy path
opt-in behind config. Don't bake Kafka (or sharding, or an inbox) into the domain.

See `services/ingest/webhook/processor.go` + its `async.go` / `http.go` /
`inbox.go` siblings for the canonical swap. → [messaging.md](messaging.md) §7.

---

## 7. One controller file per transport

A domain is reached over several transports; each gets its own file in the
`…controller/` package, all calling the same service.

```
paymentcontroller/
  http.go       # public REST handler
  internal.go   # service-to-service HTTP (HTTP webhook mode)
  kafka.go      # Kafka consumer handler (Kafka webhook mode)
```

**Why:** the transport (decode, status codes, ack semantics) is separate from the
use case. Adding "consume from Kafka" is a new file, not a rewrite of the
handler. The service stays transport-agnostic.

Ref: `services/paymanager/internal/payment/paymentcontroller/`.

---

## 8. Manual dependency injection in `app.go`

No DI framework. `NewApp` constructs everything explicitly, bottom-up: pool →
repos + repo factory → adapters → service → controllers → router.

```go
// services/silvergate/app.go (abridged)
pg, _ := postgres.New(cfg.PgURL, postgres.MaxPoolSize(10))
txRepo := transactionrepo.NewPgTransactionRepo(pg.Pool)
acq := acquirer.NewMockAcquirer(cfg.AcquirerAuthApproveRate, ...)
txRepoFactory := func(tx postgres.Executor) transaction.Repo { return transactionrepo.NewPgTransactionRepo(tx) }
svc := transaction.NewService(txRepo, acq, webhookSender, log, pg, txRepoFactory)
purchaseSvc := purchase.NewService(productSvc, svc, svc, txRepo, txRepoFactory, pg, log)
```

**Why:** wiring is explicit and greppable; the compiler catches a missing
dependency. `app.go` doubles as the architecture diagram — read it top to bottom
to see what depends on what. (Note `svc` passed twice to `purchase`: it satisfies
both the `Authorizer` and `Capturer` ports — interface segregation in action.)

Ref: `services/silvergate/app.go:42`.

## Related

- The transaction mechanics these services orchestrate → [transactions.md](transactions.md)
- Config & how `app.go` gets `cfg` → [infra-testing.md](infra-testing.md) §1
