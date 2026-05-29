# Навчальний роадмап — Interview Prep Labs

Практичний трек підготовки до Senior-співбесіди (Go + Postgres, fintech).
Пісочниця — Silvergate. На кожну тему — прив'язка до реального коду.

- **Інвентар тем:** [learning-checklist.md](learning-checklist.md)
- **Цей файл:** план — які лаби, в якому порядку, що покривають

## Принцип

Checklist розкладено на 3 типи:
- **Buildable лаб** — пишемо код у Silvergate, відтворюємо явище руками
- **Drill на готовому** — код уже є (paymanager/silvergate), треба проговорити
- **Чисте читання** — не проєкт (DDIA, System Design, SOLID)

## Лаби

Порядок: **A → B → C** (Postgres), потім **D** (Go), **E** — коли торкнемося acquirer.

| Лаб | Тема | Покриває | Передумова |
|---|---|---|---|
| A | Seed + EXPLAIN/index drills | "виконання запитів" (~9) | — |
| B | Concurrency (lost update / write skew) | "tx/MVCC/локи" (~8) | FOR UPDATE, CAS вже є |
| C | Migration-locks (safe rollout) | ALTER → ACCESS EXCLUSIVE, міграція без простою | велика таблиця з A |
| D | Go perf/load | "Go depth" (~6): pprof, GC, escape, leaks, race | loadtest + pprof |
| E | Resilience patterns | retry/backoff/circuit breaker, idempotency keys | `acquirer` port |

### Lab A — Seed + EXPLAIN/index drills
Накачати `transactions`/`products`/`refunds` до обсягу+skew де EXPLAIN ANALYZE
дає реальні плани. Дрили: прогнозуєш план → запускаєш → звіряєш.
Покриває: left-prefix, implicit cast, low-selectivity seq scan, partial index,
Nested Loop vs Hash, stale stats → ANALYZE.
План: _(TBD — lab-a-explain/plan.md)_

### Lab B — Concurrency
`refunded_amount` + concurrent partial refunds = полігон lost update / write skew.
Зламаний read-modify-write без локу → відтворити → полагодити: FOR UPDATE /
атомарний UPDATE / optimistic CAS / Serializable + retry. Write skew: два рефанди
валідні окремо, разом > captured. Діагностика: pg_locks, pg_stat_activity.
План: _(TBD)_

### Lab C — Migration-locks
На великій seeded-таблиці: наївна `ALTER COLUMN TYPE` → ACCESS EXCLUSIVE → виміряти
лок через pg_locks поки запит висить → переписати safe (CREATE INDEX CONCURRENTLY,
nullable + батчевий backfill + NOT VALID → VALIDATE, lock_timeout + retry).
План: _(TBD)_

### Lab D — Go perf/load
loadtest проти `/purchase` + pprof endpoints. Профілювати під навантаженням →
боттлнек → фікс. GC під load, escape analysis, goroutine leaks (кандидат —
webhooksender), race detector.
План: _(TBD)_

### Lab E — Resilience patterns
retry/backoff/circuit breaker навколо `acquirer` port. Idempotency keys вже є
(`/purchase` + F-α/F-β) — закріпити як розповідь.
План: _(TBD)_

## Drill на готовому

- **Масштабування:** pg_partman / Patroni / HAProxy rw-ro / etcd failover /
  CDC WAL→Kafka — у paymanager руками. Формат: навіщо / альтернатива / trade-off.
- **Observability:** Prometheus + Grafana вже є.
- **Exactly-once / idempotency:** `/purchase` + F-α/F-β.

## Чисте читання

LSM-tree vs heap, DDIA, Raft, System Design (Alex Xu), SOLID, mock-співбесіди.
