# Список для вивчення та практики

## Postgres — виконання запитів

- [ ] EXPLAIN vs EXPLAIN ANALYZE — план vs реальне виконання
- [ ] Читання плану: вузли знизу вгору, estimated vs actual rows
- [ ] Рефлекс «повільний запит → EXPLAIN ANALYZE → actual vs estimated → індекс/статистика/план» (НЕ в логи першим ділом)
- [ ] Типи scan: Seq / Index / Index Only / Bitmap Heap — і чому планувальник свідомо обирає seq scan
- [ ] Join-и: Nested Loop / Hash / Merge — коли кожен
- [ ] Індекси: B-tree, GIN (jsonb/масиви), коротко GiST / BRIN / Hash
- [ ] Composite-індекс + правило лівого префіксу
- [ ] Partial index, Covering index (INCLUDE) → Index Only Scan
- [ ] Чому індекс не використовується: функція над колонкою, неявний каст, низька селективність, застаріла статистика (ANALYZE)
- [ ] Розвести: LSM-tree (RocksDB/Cassandra) — це storage engine, НЕ індекс Postgres; Postgres = heap + B-tree

## Postgres — транзакції / MVCC / локи

- [ ] MVCC: версії рядків, dead tuples
- [ ] VACUUM / autovacuum — навіщо; table bloat — звідки
- [ ] Рівні ізоляції: Read Committed / Repeatable Read / Serializable — що кожен дозволяє
- [ ] Аномалії: lost update, write skew, phantom, non-repeatable read — на прикладі грошей
- [ ] Коли у fintech потрібен Serializable або явні локи
- [ ] Row-level locks: SELECT FOR UPDATE, FOR NO KEY UPDATE
- [ ] Deadlock: як виникає, як уникати (впорядкований доступ)
- [ ] Діагностика: pg_locks, pg_stat_activity
- [ ] Advisory locks — коли доречні
- [ ] Локи при ALTER TABLE: які беруть ACCESS EXCLUSIVE і кладуть таблицю, що безпечно
- [ ] Безпечні міграції на великих таблицях: lock_timeout, CREATE INDEX CONCURRENTLY, без простою

## Postgres — масштабування

- [ ] Партиціювання (pg_partman): partition pruning, користь і межі
- [ ] Реплікація (Patroni streaming): sync vs async, replication lag
- [ ] Read/write split (HAProxy): навіщо, ризик stale-read з репліки
- [ ] HA / failover (etcd leader election): що при падінні primary
- [ ] CDC (логічна реплікація WAL → Kafka): навіщо vs дворазовий запис
- [ ] Для кожного рішення: навіщо / альтернатива / trade-offs

## Go — gotchas

- [ ] nil-канал: читання/запис блокують навічно; трюк з вимкненням гілки в select
- [ ] Закритий канал: спершу буферизовані дані, тоді zero value + ok=false; повторний close/запис → паніка
- [ ] typed nil interface: інтерфейс ≠ nil, якщо містить typed-nil вказівник
- [ ] Graceful shutdown: signal.NotifyContext / context.WithCancel, чекати завершення воркерів
- [ ] context: deadline/cancel propagation
- [ ] Data race на map, sync.WaitGroup, errgroup

## Go — глибина для Senior

- [ ] pprof: профілювання CPU / памʼяті
- [ ] Поведінка GC під навантаженням
- [ ] Escape analysis (стек vs купа)
- [ ] Goroutine leaks: як виникають, як ловити
- [ ] Race detector на практиці
- [ ] Профілювати власний payment manager під навантаженням, знайти боттлнек, полагодити

## Distributed systems (через DDIA)

- [ ] DDIA: транзакції → consistency-моделі → consensus → проблеми розподілених систем
- [ ] Exactly-once / ідемпотентність — закріпити на вебхуках свого проєкту
- [ ] Consistency: strong vs eventual, коли що
- [ ] Consensus: Raft (на рівні ідеї, бо вже маєш etcd)
- [ ] Патерни відмовостійкості: retry, backoff, circuit breaker, idempotency keys

## System design

- [ ] System Design Interview (Alex Xu) — тренування формату
- [ ] Структуровано вести дизайн за ~45 хв
- [ ] Mock-співбесіди (Pramp / з колегами)

## Спостережуваність

- [ ] Tracing, метрики, structured logging, SLO
- [ ] Закріпити на проєкті (вже є Prometheus + Grafana)

## SOLID

- [ ] Назви + 1 приклад на кожен принцип

## Ресурси

- [ ] DDIA — дочитати
- [ ] PostgreSQL docs: MVCC, Indexes, Explicit Locking
- [ ] use-the-index-luke.com — індекси й плани
- [ ] «PostgreSQL 14 Internals» (Rogov) — глибше в MVCC, опційно
- [ ] pgexercises.com — практика SQL
- [ ] System Design Interview (Alex Xu)
