# PostgreSQL HA & DR: Streaming Replication

**Status:** In Progress

## Overview

Practice PostgreSQL streaming replication with a primary-standby setup and application-level read replica routing. Focus on practical understanding of replication mechanics, not production-grade HA tooling.

**Scope (intentionally lightweight):**
- Docker Compose with primary + async standby replica
- Application-level read/write split (writes → primary, reads → replica)
- Observe replication lag, read-after-write consistency issues

**Progression:** Start simple (manual replication, app-level routing), then build up to production patterns (Patroni, PITR, lag monitoring).

## Key Concepts to Practice

- **Streaming replication** — WAL shipping from primary to standby, hot standby for read queries
- **Synchronous vs asynchronous** — trade-offs between durability and latency
- **Read/write split** — separate connection pools, routing by query type
- **Read-after-write consistency** — the fundamental problem with read replicas
- **Replication lag** — what causes it, how to observe it

## Architecture

```
                    ┌─────────┐
                    │  etcd   │  ← leader election
                    └────┬────┘
                         │
          ┌──────────────┼──────────────┐
          │              │              │
    ┌─────┴─────┐  ┌─────┴─────┐  ┌─────┴─────┐
    │ patroni1  │  │ patroni2  │  │ patroni3  │
    │ PG + Pat  │  │ PG + Pat  │  │ PG + Pat  │
    │ :8008 API │  │ :8008 API │  │ :8008 API │
    └───────────┘  └───────────┘  └───────────┘
          │              │              │
          └──────────────┼──────────────┘
                         │
App (API) ──rw pool──→ HAProxy :5440 ──→ /primary (httpchk)
          ──ro pool──→ HAProxy :5441 ──→ /replica (httpchk, round-robin)
```

## Tasks

- [x] Subtask 1: Streaming replication setup — Docker Compose with primary + standby, verify replication works
  - **Plan:** [plan-subtask-1.md](plan-subtask-1.md)
- [x] Subtask 2: Read replica routing — HAProxy (rw/ro endpoints), 2 replicas, app-level read/write split at repository level
  - **Plan:** [plan-subtask-2.md](plan-subtask-2.md)
- [x] Subtask 3: Failover/switchover — manual promotion, automated failover with Patroni basics
  - **Plan:** [plan-subtask-3.md](plan-subtask-3.md)
  - Reference: [HA PostgreSQL with Patroni and HAProxy](https://jfrog.com/community/devops/highly-available-postgresql-cluster-using-patroni-and-haproxy/)
- [ ] Subtask 4: Backup/restore — pg_basebackup for PITR, WAL archiving, restore verification
- [x] Subtask 5a: Monitoring — replication lag metrics, HAProxy metrics, postgres-exporter, Grafana dashboard
- [ ] Subtask 5b: Monitoring — backup success/failure alerts
- [ ] Subtask 6: Replication lag consistency test — demonstrate read-after-write inconsistency with read replicas

## Useful Links

- HAProxy stats: http://localhost:8404/stats
- Grafana dashboards: http://localhost:3100
  - Service Health: http://localhost:3100/d/service-health
  - PostgreSQL HA: http://localhost:3100/d/postgres-ha
- Prometheus: http://localhost:9090
- Prometheus targets: http://localhost:9090/targets

## Notes

- Async replication by default (simpler, shows lag naturally)
- Reuse existing `PG.Dockerfile` as base for primary
- Standby uses `pg_basebackup` for initial sync, then streams WAL
- Application routing: new `PG_REPLICA_URL` env var, second `pgxpool.Pool`

## Implementation Log

### Subtask 1: Streaming Replication Setup
- Docker Compose: `db-primary` (port 5432) + `db-replica` (port 5433) under `replication` profile
- `scripts/init-db.sh` creates `replicator` role with `REPLICATION LOGIN` on primary
- `scripts/init-replica.sh` runs `pg_basebackup` from primary, starts replica with `hot_standby=on`
- Primary configured with `wal_level=logical`, `max_wal_senders=10`
- Verified: `pg_stat_replication` shows connected standby, writes on primary visible on replica

### Subtask 2: Read Replica Routing via HAProxy
- **Removed standalone `db` service** — `db-primary` is now the single database in all profiles (`infra`, `prod`)
- Added `db-replica-2` (port 5434) — second replica for round-robin
- **HAProxy** (`config/haproxy.cfg`): TCP proxy, rw frontend `:5440` → primary, ro frontend `:5441` → replicas round-robin, stats at `:8404/stats`
- HAProxy uses Docker DNS resolver (`127.0.0.11`) with `init-addr none` — starts even if backends aren't ready yet
- **Config**: optional `PG_REPLICA_URL` in `APIConfig` — empty means all reads go to primary
- **App wiring** (`app.go`): `var readDB postgres.Executor = pool.Pool` by default, overridden with replica pool if `PG_REPLICA_URL` set. Single `readDB` passed to all repos/eventsinks
- **Repos**: `db` field for writes, `readDB` field for reads. No nil checks — `readDB` always set (either replica or same as primary). `TxRepoFactory` passes `readDB: tx` so reads within transactions stay on the same tx
- **Go nil interface gotcha**: can't assign typed nil `*pgxpool.Pool` to `postgres.Executor` interface — it becomes non-nil. Solved by always providing a valid executor, never nil
- `pkg/postgres` struct unchanged — no `ReadPool` field. Read pool created via same `postgres.New()` and passed explicitly
- `endpoints.host.env` updated: `PG_URL` → `:5440`, `PG_REPLICA_URL` → `:5441`
- HAProxy stats page (`http://localhost:8404/stats`) shows backend health and session distribution
- **Monitoring**: merged `monitoring` profile into `infra` — Prometheus, Grafana, postgres-exporter all start with `make start_containers`
- **postgres-exporter** — scrapes `db-primary` for `pg_stat_replication`, `pg_stat_activity`, database size
- **HAProxy Prometheus endpoint** — `/metrics` on `:8404` via `prometheus-exporter` service
- **Grafana dashboard** `postgres-ha` — replication lag, HAProxy sessions/bytes/status, PG connections, DB size
- **Loadtest** updated — sends GET queries (orders, events) alongside webhooks to exercise read replicas
- **Analytics retry** — `newIndexer` retries up to 60s waiting for OpenSearch startup

### Subtask 3: Patroni + etcd — Automated Failover
- **Replaced manual replication** with Patroni cluster manager — all 3 PG nodes identical, Patroni decides roles
- **etcd** (1 node) as DCS — stores leader key, used for leader election
- **PG.Dockerfile** — added `patroni[etcd3]` via pip, custom entrypoint (`patroni-entrypoint.sh`) ensures data dir permissions
- **`config/patroni.yml`** — single config for all nodes: bootstrap params, pg_hba, replication auth, `use_pg_rewind`, `use_slots`
- Node-specific settings (`name`, `connect_address`) via env vars in docker-compose (`PATRONI_NAME`, `PATRONI_RESTAPI_CONNECT_ADDRESS`, `PATRONI_POSTGRESQL_CONNECT_ADDRESS`)
- **`scripts/post-bootstrap.sh`** — creates `payments` and `ingest` databases on initial leader bootstrap
- **HAProxy** — switched from `pgsql-check` to Patroni REST API health checks (`/primary`, `/replica` on port 8008)
- All 3 nodes listed in both backends — HAProxy determines who is primary/replica via health checks
- `on-marked-down shutdown-sessions` — immediately drops connections on failover
- **docker-compose** — `--wait` flag ensures `start_containers` returns only when all Patroni healthchecks pass
- **Deleted** `scripts/init-db.sh`, `scripts/init-replica.sh` — Patroni handles bootstrap, replication user, pg_basebackup
- **Failover tested**: `docker stop <leader>` → new leader elected in ~10-15s, HAProxy reroutes automatically
- **17% errors during failover** are expected — async replication means ~2-5s unavailability window
- After failover, stopped node rejoins as replica via `pg_rewind` (no full basebackup needed)
