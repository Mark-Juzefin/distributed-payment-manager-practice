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
Current:
  App (API) ──write+read──→ PostgreSQL (single)

With replication:
  App (API) ──writes──→ PostgreSQL Primary ──WAL stream──→ PostgreSQL Replica
             ──reads───→ PostgreSQL Replica
```

## Tasks

- [x] Subtask 1: Streaming replication setup — Docker Compose with primary + standby, verify replication works
  - **Plan:** [plan-subtask-1.md](plan-subtask-1.md)
- [ ] Subtask 2: Read replica routing — application-level read/write split, observe consistency trade-offs
- [ ] Subtask 3: Failover/switchover — manual promotion, automated failover with Patroni basics
- [ ] Subtask 4: Backup/restore — pg_basebackup for PITR, WAL archiving, restore verification
- [ ] Subtask 5: Monitoring — replication lag metrics, backup success/failure alerts

## Notes

- Async replication by default (simpler, shows lag naturally)
- Reuse existing `PG.Dockerfile` as base for primary
- Standby uses `pg_basebackup` for initial sync, then streams WAL
- Application routing: new `PG_REPLICA_URL` env var, second `pgxpool.Pool`
