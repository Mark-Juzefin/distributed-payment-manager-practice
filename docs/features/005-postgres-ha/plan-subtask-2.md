# Plan: Subtask 2 — Read Replica Routing via HAProxy

## Context
Subtask 1 gave us a standalone primary + replica demo. Now we integrate replication into the default dev workflow: HAProxy proxies rw/ro traffic, the app has two connection pools, and repositories route reads to replicas.

## Current State
- `db-primary` + `db-replica` under Docker Compose profile `replication` (standalone demo)
- `db` service under profiles `infra`/`prod` — current default for `make run-dev`
- App uses single `*postgres.Postgres` (one `pgxpool.Pool`) for everything
- Repos have embedded `repo` struct with `db postgres.Executor` — all ops go through it
- Event sinks take `postgres.Executor` directly

## Architecture

```
App (API) ──writes──→ HAProxy :5440 (rw) ──→ db-primary
           ──reads───→ HAProxy :5441 (ro) ──→ db-replica (round-robin)
                                            ──→ db-replica-2
```

## Key Decision: Replication as Default

Replication becomes part of the `infra` profile — `make run-dev` starts primary + 2 replicas + HAProxy.

- Old `db` service → replaced by `db-primary` (same config, just renamed)
- `db-replica`, `db-replica-2` added to `infra` profile
- `haproxy` added to `infra` profile
- `endpoints.host.env` updated: `PG_URL` → HAProxy rw port (5440), new `PG_REPLICA_URL` → HAProxy ro port (5441)
- No separate `Procfile.replication` or `make start-replication` — the regular `Procfile` and `make run-dev` just work
- Old standalone `replication` profile services removed (superseded)

## Implementation Steps

### 1. Replace `db` with replication cluster in `docker-compose.yaml`

**`docker-compose.yaml`**:

- Remove old `db` service (profiles `infra`/`prod`)
- Move `db-primary` from `replication` to `infra`/`prod` profiles, keep port 5432
- Add `db-replica-2`: same as `db-replica` but port 5434, own volume
- Move `db-replica` to `infra`/`prod` profiles
- Add `haproxy` service to `infra`/`prod` profiles:
  - image `haproxy:2.9`
  - ports 5440 (rw), 5441 (ro), 8404 (stats)
  - depends on `db-primary` healthy
- Remove `replication` profile (no longer needed)
- Update `api-service` and other services that depend on `db` → depend on `haproxy` instead
- Add volumes: `db-primary-data` (rename from `db-data`), `db-replica-data`, `db-replica-2-data`

### 2. Create HAProxy config

**`config/haproxy.cfg`** (new file):

```
global
    maxconn 100

defaults
    mode tcp
    timeout connect 5s
    timeout client  30s
    timeout server  30s
    timeout check   5s

listen stats
    bind *:8404
    mode http
    stats enable
    stats uri /stats
    stats refresh 5s

frontend pg_rw
    bind *:5440
    default_backend pg_primary

frontend pg_ro
    bind *:5441
    default_backend pg_replicas

backend pg_primary
    option pgsql-check user postgres
    server db-primary db-primary:5432 check inter 3s fall 3 rise 2

backend pg_replicas
    balance roundrobin
    option pgsql-check user postgres
    server db-replica db-replica:5432 check inter 3s fall 3 rise 2
    server db-replica-2 db-replica-2:5432 check inter 3s fall 3 rise 2
```

### 3. Update env files

**`env/endpoints.host.env`** — update PG_URL, add PG_REPLICA_URL:
```
PG_URL=postgres://postgres:secret@localhost:5440/payments?sslmode=disable
PG_REPLICA_URL=postgres://postgres:secret@localhost:5441/payments?sslmode=disable
INGEST_PG_URL=postgres://postgres:secret@localhost:5440/ingest?sslmode=disable
```

**`env/endpoints.docker.env`** — update for Docker-internal HAProxy:
```
PG_URL=postgres://postgres:secret@haproxy:5440/payments?sslmode=disable
PG_REPLICA_URL=postgres://postgres:secret@haproxy:5441/payments?sslmode=disable
```

### 4. Add `PG_REPLICA_URL` to config

**per-service `config/config.go`** — add to `APIConfig`:
```go
PgReplicaURL string `env:"PG_REPLICA_URL"` // optional, fallback to PG_URL
```

Not `required` — when empty, all reads go to primary (backward compatible for tests, standalone runs).

### 5. Add read pool to `pkg/postgres`

**`pkg/postgres/postgres.go`**:

- Add `ReadPool *pgxpool.Pool` field to `Postgres` struct
- Add `ReadExecutor() Executor` method — returns `ReadPool` if set, else `Pool`
- Update `Close()` to close `ReadPool`
- Add `NewReadPool(url string, opts ...Option) (*pgxpool.Pool, error)` — returns `nil, nil` if url is empty

### 6. Add `readDB` to repositories

Both `order_repo.repo` and `dispute_repo.repo` get a `readDB postgres.Executor` field and a `reader()` helper:

```go
type repo struct {
    db      postgres.Executor  // primary (writes + tx reads)
    readDB  postgres.Executor  // replica (standalone reads)
    builder squirrel.StatementBuilderType
}

func (r *repo) reader() postgres.Executor {
    if r.readDB != nil {
        return r.readDB
    }
    return r.db
}
```

**Read methods** switch from `r.db.Query(...)` to `r.reader().Query(...)`:
- Order: `GetOrders`
- Dispute: `GetDisputes`, `GetDisputeByID`, `GetDisputeByOrderID`, `GetEvidence`

**Write methods** stay on `r.db` — no changes.

**Transaction repos** (`TxRepoFactory`) don't set `readDB` → `reader()` falls back to `r.db` (the tx) — correct.

**Constructor changes:**
```go
func NewPgOrderRepo(pg *postgres.Postgres) order.OrderRepo {
    return &PgOrderRepo{
        pg:   pg,
        repo: repo{db: pg.Pool, readDB: pg.ReadPool, builder: pg.Builder},
    }
}
```
Same for `NewPgDisputeRepo`.

### 7. Add `readDB` to event sinks

Same pattern for `PgOrderEventRepo` and `PgDisputeEventRepo`:

- Add `readDB postgres.Executor` field + `reader()` helper
- Read methods (`GetOrderEvents`, `GetOrderEventByID`, `GetDisputeEvents`, `GetDisputeEventByID`) use `r.reader()`
- Constructor adds `readDB` param: `NewPgOrderEventRepo(db, readDB postgres.Executor, builder)`
- `PgEventStore` (outbox) is write-only → no changes

### 8. Wire up in `app.go`

```go
pool, err := postgres.New(cfg.PgURL, postgres.MaxPoolSize(cfg.PgPoolMax))
// ...

readPool, err := postgres.NewReadPool(cfg.PgReplicaURL, postgres.MaxPoolSize(cfg.PgPoolMax))
// ...
pool.ReadPool = readPool
if readPool != nil {
    defer readPool.Close()
    slog.Info("Read replica pool configured")
    healthCheckers = append(healthCheckers, health.NewPostgresChecker(readPool))
}

// Repos pick up ReadPool via pg.ReadPool
orderRepo := order_repo.NewPgOrderRepo(pool)
disputeRepo := dispute_repo.NewPgDisputeRepo(pool)

// Event sinks need explicit read executor
var readExec postgres.Executor
if readPool != nil {
    readExec = readPool
}
disputeEvents := dispute_eventsink.NewPgEventRepo(pool.Pool, readExec, pool.Builder)
orderEvents := order_eventsink.NewPgOrderEventRepo(pool.Pool, readExec, pool.Builder)
```

### 9. Makefile cleanup

- Remove `run-replication` and `stop-replication` targets (no longer needed)
- `start_containers` already does `docker-compose --profile infra up --build -d` — works as-is
- `make run-dev` works unchanged

### 10. Fix existing tests

Event sink constructors gain a `readDB` param. Existing tests pass `nil`:
- `NewPgOrderEventRepo(db, nil, builder)`
- `NewPgEventRepo(db, nil, builder)`

## Files to Modify

| File | Change |
|------|--------|
| `docker-compose.yaml` | Replace `db` with `db-primary` + replicas + `haproxy` in `infra`/`prod` |
| `config/haproxy.cfg` | **New** — HAProxy rw/ro config |
| per-service `config/config.go` | Add `PgReplicaURL` to `APIConfig` |
| `pkg/postgres/postgres.go` | Add `ReadPool`, `ReadExecutor()`, `NewReadPool()`, update `Close()` |
| `services/api/repo/order/pg_order_repo.go` | Add `readDB` + `reader()`, update `GetOrders`, constructors |
| `services/api/repo/dispute/pg_dispute_repo.go` | Add `readDB` + `reader()`, update read methods, constructors |
| `services/api/repo/order_eventsink/pg_order_event_sink.go` | Add `readDB` + `reader()`, update read methods, constructor |
| `services/api/repo/dispute_eventsink/pg_dispute_event_sink.go` | Same as above |
| `services/api/app.go` | Create read pool, wire to repos + event sinks, health check |
| `env/endpoints.host.env` | Update `PG_URL` to HAProxy rw, add `PG_REPLICA_URL` |
| `env/endpoints.docker.env` | Update `PG_URL` to HAProxy rw, add `PG_REPLICA_URL` |
| `Makefile` | Remove `run-replication`/`stop-replication` |
| Tests (eventsink integration) | Pass `nil` for new `readDB` param |

## Verification

```bash
# 1. Start infra (now includes replication + HAProxy)
make start_containers

# 2. Check HAProxy stats
open http://localhost:8404/stats
# Expected: pg_primary (1 green), pg_replicas (2 green)

# 3. Start app as usual
make run-dev

# 4. Generate traffic
make loadtest

# 5. Check HAProxy stats: pg_replicas should show increasing session count

# 6. Unit tests still pass
make test

# 7. Integration tests still pass (nil readDB)
make integration-test
```
