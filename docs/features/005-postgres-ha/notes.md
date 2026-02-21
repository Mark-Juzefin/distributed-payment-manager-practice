# Notes & Observations

## Replication

- **Physical streaming replication** (WAL-based) — all tables replicated, not selective
- Async by default — primary doesn't wait for replica ACK, so there's inherent replication lag
- Replicas are hot standbys — accept read-only queries while streaming WAL

## HAProxy as TCP Proxy

- Operates at **TCP level (Layer 4)** — doesn't parse SQL, just forwards connections
- App-level routing: the application decides which pool (rw/ro) to use per query
- Alternative: **Pgpool-II** parses SQL and routes SELECT/INSERT automatically, but adds complexity and latency
- HAProxy health check: `option pgsql-check` — sends PG startup message, verifies backend is alive

## WAL Senders under Load

`pg_stat_replication` shows 3 active WAL senders on primary:

| Client | Type | Lag | Notes |
|--------|------|-----|-------|
| db-replica | physical | 0 | Fully caught up |
| db-replica-2 | physical | 0 | Fully caught up |
| CDC (`cdc_slot`) | logical | ~4 min | Decoding WAL into logical changes is slower |

- Physical replicas just copy raw WAL bytes — cheap, near-zero lag
- Logical replication (CDC) decodes WAL into INSERT/UPDATE/DELETE — more expensive, lags under load
- Confirmed via `pg_replication_slots`: `cdc_slot` is `logical`, `active = t`

## Load Test: Read/Write Split via HAProxy

**Setup:** 10 VUs, ~1.5 min, loadtest sends webhooks (writes) + GET queries (reads)

### HAProxy Stats

| Backend | Sessions | Bytes In | Bytes Out |
|---------|----------|----------|-----------|
| db-primary | 5 | 22MB | 5.7MB |
| db-replica | 1 | 972KB | 398MB |
| db-replica-2 | 1 | 971KB | 398MB |

### Observations

- **Round-robin works** — replicas get equal traffic (~398MB out each)
- **Replicas serve ~70x more data** than primary (reads return lists, writes return small acks)
- **Low session count is normal** — `pgxpool` holds persistent TCP connections, HAProxy counts TCP sessions not SQL queries
- **Bytes In on replicas is small** — SELECT queries are lightweight, responses are large

## Monitoring Setup

- **`monitoring` profile merged into `infra`** — everything starts with `make start_containers`
- **postgres-exporter** (`:9187`) — scrapes `db-primary` for `pg_stat_replication`, `pg_stat_activity`, `pg_database_size_bytes`
- **HAProxy** exposes Prometheus metrics at `:8404/metrics` via built-in `prometheus-exporter` service
- **Grafana dashboard** `postgres-ha` — 8 panels: replication lag (bytes), WAL senders, HAProxy sessions/bytes/status, PG connections, DB size
- **Datasource uid** must be set explicitly in provisioning (`uid: prometheus`), otherwise Grafana generates random uid and dashboard can't find it


## Patroni + etcd

### How Patroni Works

- **Single binary** that wraps PostgreSQL — starts, stops, configures, and monitors PG
- **DCS (etcd)** holds the leader key — only one node can hold it at a time
- Every `loop_wait` (10s) each node checks: am I still leader? Should I promote?
- **Bootstrap**: first node does `initdb` + `post_bootstrap` script. Others do `pg_basebackup` from leader
- **Failover**: leader dies → DCS TTL expires → replica with least lag promotes → Patroni updates etcd → HAProxy detects via REST API

### Key Config Decisions

| Setting | Value | Why |
|---------|-------|-----|
| `use_pg_rewind: true` | Allows old primary to rejoin as replica without full basebackup | Requires `data-checksums` (set in initdb) |
| `use_slots: true` | Patroni manages replication slots automatically | Prevents WAL removal before replica consumes it |
| `maximum_lag_on_failover: 1MB` | Don't promote replica that's >1MB behind | Prevents data loss on async replication |
| `basebackup: checkpoint: fast` | Force immediate checkpoint before basebackup | Default `spread` waits up to 5min for natural checkpoint |
| `on-marked-down shutdown-sessions` | HAProxy kills connections to demoted primary | Prevents writes to old primary during failover |

### Patroni YAML vs Env Vars

- Patroni does **NOT** expand `${VAR}` in YAML — treats it as literal string
- Node-specific settings must use Patroni's env var mapping:
  - `PATRONI_NAME` → `name`
  - `PATRONI_RESTAPI_CONNECT_ADDRESS` → `restapi.connect_address`
  - `PATRONI_POSTGRESQL_CONNECT_ADDRESS` → `postgresql.connect_address`
- DCS settings (`bootstrap.dcs`) are stored in etcd, shared across all nodes
- Local `postgresql.parameters` are per-node, merged with DCS params

### HAProxy Health Checks

- Old: `option pgsql-check` — only checks if PG is alive, doesn't know who is leader
- New: `option httpchk` + `http-check send meth GET uri /primary` — asks Patroni REST API
- `/primary` returns 200 only on the leader, `/replica` returns 200 only on replicas
- HAProxy 2.x syntax: `option httpchk` (no args) + `http-check send` (separate directive)
- `check port 8008` — health check goes to Patroni API, data traffic goes to PG on 5432

### Docker-Specific Gotchas

| Issue | Cause | Fix |
|-------|-------|-----|
| `initdb: cannot be run as root` | Patroni entrypoint bypasses postgres image's `gosu` | `USER postgres` in Dockerfile |
| `Cannot rename data directory` | Volume mount root can't be renamed | Use subdirectory: `data_dir: .../data/patroni` |
| `no pg_hba.conf entry for [local]` | post-bootstrap uses unix socket, pg_hba only had `host` rules | Add `local all all trust` to pg_hba |
| `pg_partman function does not exist` | post-bootstrap created extension in `public`, migration expects `partman` schema | Don't create pg_partman in post-bootstrap, let migration handle it |
| `pg_basebackup` hangs for minutes | Default `--checkpoint=spread` waits up to `checkpoint_timeout` (5min) | `basebackup: checkpoint: fast` in patroni.yml |
| Data dir permissions 755 | Docker volume mount creates parent as 1777, mkdir inherits umask | Entrypoint script: `chmod 0700 $PGDATA` at runtime |
| App gets EOF on migrations | `make run-dev` rebuilds images (`--build`), recreates containers, app starts before cluster ready | `docker-compose up --wait` + healthchecks on Patroni nodes |

### Failover Test Results

```
# Before failover (patroni1 = leader)
make loadtest → 1121 req/s, 0% errors

# docker stop patroni1 → new leader elected ~10-15s
make loadtest → 1121 req/s, 17% errors (during transition)

# After stabilization
make loadtest → 0% errors, same throughput
```

- 17% errors during failover = connections to old primary dropped by HAProxy
- Kafka consumers auto-retry failed messages
- Old primary rejoins as replica after `docker start` (pg_rewind, no full basebackup)

❯ make loadtest
go run ./loadtest -target http://localhost:3001 -vus 10 -duration 30s
Starting load test: 10 VUs, 30s
Target: http://localhost:3001
Dispute ratio: 30%


========== SUMMARY ==========
Duration:    30.009s
Requests:    30370 (1012.0/s)
Orders:    12852
Disputes:  1969
Reads:     15549
Success:     25188
Errors:      5182 (17.06%)
Latency:     p50=5.477042ms  p95=14.269ms  p99=22.884875ms
==============================