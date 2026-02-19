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
