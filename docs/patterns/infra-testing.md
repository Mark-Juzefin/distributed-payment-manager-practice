# Infrastructure & Testing

The plumbing every service shares — config, the Postgres pool, embedded
migrations — and how the code is tested: hand-rolled fakes for fast unit tests,
generated mocks for ports, and testcontainers for integration.

Canonical source: `*/config/config.go`, `pkg/postgres/`, `pkg/migrations/`,
`pkg/testinfra/`, `*/internal/**/*_test.go`.

---

## 1. Config via struct tags (`caarlos0/env`)

Each service has its own `config.Config` populated from the environment. Missing
required vars fail startup; everything else has a default.

```go
// services/silvergate/config/config.go
type Config struct {
    Port     int    `env:"PORT" envDefault:"3002"`
    PgURL    string `env:"SILVERGATE_PG_URL" required:"true"`
    LogLevel string `env:"LOG_LEVEL" envDefault:"info"`

    WebhookCallbackURL string  `env:"WEBHOOK_CALLBACK_URL" required:"true"`
    AcquirerAuthApproveRate float64 `env:"ACQUIRER_AUTH_APPROVE_RATE" envDefault:"0.9"`
    AcquirerSettleDelay     time.Duration `env:"ACQUIRER_SETTLE_DELAY" envDefault:"500ms"`
}

func New() (Config, error) { return env.ParseAs[Config]() }
```

**Why:** config is a typed struct, not loose `os.Getenv` calls scattered around.
`required:"true"` turns a missing var into a clear boot failure instead of a
zero-value bug. `time.Duration` parses `"500ms"` for free.

Ref: `services/silvergate/config/config.go`.

---

## 2. Postgres: pooled pgx + Squirrel, with connect retry

`pkg/postgres` wraps `pgxpool`, exposes a Squirrel builder configured for
Postgres `$N` placeholders, and retries the initial connect (containers come up
slowly). Tunables use the functional-options pattern.

```go
// pkg/postgres/postgres.go
pg, err := postgres.New(cfg.PgURL, postgres.MaxPoolSize(10))   // Option funcs
// pg.Pool    → *pgxpool.Pool (implements Executor)
// pg.Builder → squirrel.StatementBuilder.PlaceholderFormat(squirrel.Dollar)
```

Repos build SQL with Squirrel and run it on the `Executor`:

```go
var psql = sq.StatementBuilder.PlaceholderFormat(sq.Dollar)
query, args, _ := psql.Select(cols...).From("transactions").Where(sq.Eq{"id": id}).ToSql()
row := r.db.QueryRow(ctx, query, args...)
```

**Why:** one connection-management spot; the `Executor` seam (see
[transactions.md](transactions.md) §1) means repos don't care if `db` is the pool
or a tx. Squirrel keeps SQL type-safe and composable (e.g. add `.Suffix("FOR
UPDATE")` for a lock).

Refs: `pkg/postgres/postgres.go:31` (New), `:43` (builder), `pkg/postgres/options.go` (Options).

---

## 3. Embedded Goose migrations, applied on startup

Migrations live next to each service as `.sql` files, embedded with `embed.FS`,
and run automatically when the app boots. The same FS is reused by integration
tests.

```go
// services/silvergate/app.go
//go:embed migrations/*.sql
var migrationFS embed.FS
func MigrationFS() embed.FS { return migrationFS }   // exported for tests

// in NewApp, before opening the pool:
if err := migrations.ApplyMigrations(cfg.PgURL, migrationFS); err != nil { ... }
```

```go
// pkg/migrations/goose.go
func ApplyMigrations(connStr string, migrationFS embed.FS) error {
    db, _ := sql.Open("postgres", connStr)
    goose.SetBaseFS(migrationFS)
    goose.SetDialect("postgres")
    return goose.Up(db, "migrations")
}
```

File format (`YYYYMMDDNNN_name.sql`, up + down in one file):

```sql
-- +goose Up
-- +goose StatementBegin
CREATE TABLE transactions ( ... );
-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
DROP TABLE transactions;
-- +goose StatementEnd
```

**Why:** schema travels *inside* the binary (`embed.FS`) — no separate migration
step to forget, dev and prod converge. Exporting `MigrationFS()` lets tests spin
up the exact production schema.

Refs: `pkg/migrations/goose.go`, `services/silvergate/migrations/`, `services/silvergate/app.go:29`.

---

## 4. Unit tests with hand-rolled fakes

Service tests use small hand-written fakes (not generated mocks) that record what
was called. The transactor fake just invokes the callback with a `nil`
`Executor`, so transaction *composition* is tested without a database.

```go
// purchase/service_test.go
type fakeTransactor struct{}
func (fakeTransactor) InTransaction(_ context.Context, _ pgx.TxIsoLevel, fn func(postgres.Executor) error) error {
    return fn(nil)            // run the body; repos are faked, so nil Executor is fine
}

type fakeProductService struct { markCalled bool; markErr error /* ... */ }
func (f *fakeProductService) MarkPurchasedInTx(_ context.Context, _ postgres.Executor, _ string, _ uuid.UUID) error {
    f.markCalled = true; return f.markErr
}
```

Tests then assert **behaviour and side effects**, e.g. "on decline, neither the
product mark nor the capture runs":

```go
if products.markCalled { t.Error("MarkPurchasedInTx should not be called on decline") }
if cap.called          { t.Error("Capture should not be called on decline") }
```

**Why:** fakes that fulfil the [ddd-structure.md](ddd-structure.md) §5 ports give
millisecond, deterministic tests of the *orchestration logic* (idempotency
replay, race resolution, rollback-on-error, partial-failure signalling) with no
container. The fake `Transactor` is the key trick: it exercises the real
in-tx code path without a real tx.

Ref: `services/silvergate/internal/purchase/service_test.go` (full matrix: happy path, decline, archived, cache-hit, insert-race, capture-failure, mark-rollback).

---

## 5. Generated mocks for ports (`mockgen`)

Where a port is broad or stable, mocks are generated (`uber/mock`) rather than
hand-written, via `go:generate` directives, and committed as `mock_*.go`.

```
acquirer/mock_acquirer.go        # mock for the Acquirer port
ingest/apiclient/mock_client.go  # mock for the API HTTP client
ingest/repo/inbox/mock_inbox_repo.go
```

Regenerate with `make generate` (`go generate ./...`).

**Why:** hand fakes win for orchestration tests you want to read; generated mocks
win for wide interfaces and call-order/expectation assertions. The repo uses
each where it fits — neither is mandated everywhere.

---

## 6. Integration tests with testcontainers

Tagged `//go:build integration` so `make test` (unit) stays fast. `TestMain`
boots a real Postgres container, applies the **production** migrations via the
exported `MigrationFS()`, runs the suite, then cleans up.

```go
//go:build integration
package transactionrepo_test

func TestMain(m *testing.M) {
    ctx := context.Background()
    pgContainer, err := testinfra.NewPostgresWithConfig(ctx, testinfra.PostgresConfig{
        DBName:      "silvergate_tx_test",
        MigrationFS: silvergate.MigrationFS(),     // same schema as prod
        Image:       "postgres:17",
    })
    if err != nil { panic(err) }
    pg = pgContainer.Pool
    code := m.Run()
    pgContainer.Cleanup(ctx)
    os.Exit(code)
}
```

`pkg/testinfra` provides container helpers for Postgres, Kafka, Wiremock, and a
combined E2E suite. Run with `make integration-test`.

**Why:** repository SQL, constraints, and `FOR UPDATE`/`SKIP LOCKED` semantics
can't be verified against a mock — they need a real engine. The build tag keeps
that cost out of the inner loop.

Refs: `services/silvergate/internal/transaction/transactionrepo/integration_test.go`, `pkg/testinfra/postgres.go`, `pkg/testinfra/suite.go`.

---

## 7. Project rule: every constraint-bearing migration gets an integration test

A standing rule in this repo: a migration that adds a UNIQUE / FK / CHECK
constraint, an index that changes query behaviour, or partitioning **must** be
covered by an integration test that proves the constraint fires and maps to the
right domain error.

Partitioned-table gotcha (here `dispute_events` uses `pg_partman` daily
partitions): a UNIQUE constraint **must include the partition key**, and the test
must use the **same** partition-key value on the duplicate to trigger the
violation.

```sql
-- ✗ fails on a partitioned table
CREATE UNIQUE INDEX idx ON dispute_events(entity_id, provider_event_id);
-- ✓ include the partition column
CREATE UNIQUE INDEX idx ON dispute_events(entity_id, provider_event_id, created_at);
```

**Why:** constraints are correctness guarantees; an untested one may silently not
fire. The test documents what the constraint protects and catches regressions.

Refs: `.claude/rules/migrations.md`, `services/paymanager/internal/*/...eventsink*integration_test.go`.

## Related

- The `Executor` abstraction the pool/tx share → [transactions.md](transactions.md) §1
- The ports these fakes/mocks implement → [ddd-structure.md](ddd-structure.md) §5
