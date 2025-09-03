# Postgres Time Series Partitioning

I have a `dispute_events` table with **5,000,000** rows evenly distributed over a **one-month** time range by `created_at`. I want to query events for some disputes in **multi-day windows**. I will now explore ways to optimize this query.

### Schema

```sql
CREATE TABLE IF NOT EXISTS "dispute_events" (
    id VARCHAR(255) PRIMARY KEY,
    dispute_id VARCHAR(255) NOT NULL,
    kind VARCHAR(32) NOT NULL,
    provider_event_id VARCHAR(255) NOT NULL,
    data JSONB NOT NULL,
    created_at TIMESTAMP NOT NULL,
    CONSTRAINT fk_dispute_event_dispute FOREIGN KEY (dispute_id) REFERENCES disputes(id)
)
```

Seed table with data

```sql
select count (*) from dispute_events ;
  count
---------
 5000000
(1 row)
```

### Quering events

Now I select events with `kind = 'evidence_added'` for disputes that are **lost + fraud** in a **5-day** window. The execution takes **≈0.78 s.** The plan shows `Buffers: shared read = 105k` - That’s a huge number of pages for just 1,044 rows. With 8-KB pages, that’s about **0.83 GB** read. That’s a lot of I/O, so we need to cut it with a composite B-tree index on `(kind, created_at)` **.**

```sql
EXPLAIN (ANALYZE, COSTS off, BUFFERS) 
SELECT de.* 
FROM dispute_events AS de
JOIN disputes AS d ON d.id = de.dispute_id
WHERE de.created_at >= '2025-08-10'
  AND de.created_at <  '2025-08-15'
  AND de.kind = 'evidence_added'
  AND d.status = 'lost'
  AND d.reason = 'fraud';
```

Query plan (no useful index on `dispute_events`):

```sql
Gather (actual time=77.032..773.734 rows=1044 loops=1)
   Workers Planned: 2
   Workers Launched: 2
   Buffers: shared hit=13283 read=105894
   ->  Parallel Hash Join ...
         ->  Parallel Seq Scan on dispute_events de
		          (actual time=9.616..634.008 rows=9804 loops=3)
               Rows Removed by Filter: 1656863
               Buffers: shared hit=13103 read=105894
         ...

Execution Time: 781.735 ms
```

So I added a B-tree index on `(kind, created_at)` to `dispute_events`

```sql
CREATE INDEX IF NOT EXISTS de_kind_created_at_inc_dispute
    ON public.dispute_events (kind, created_at);
```

Same query on the indexed table:

```sql
Hash Join (actual time=45.091..522.185 rows=1044 loops=1)
   Hash Cond: ((de.dispute_id)::text = (d.id)::text)
   Buffers: shared hit=1 read=26415
   ->  Bitmap Heap Scan on dispute_events de 		
			   (actual time=15.058..476.748 rows=29411 loops=1)
         Heap Blocks: exact=26155
         Buffers: shared read=26304
         ->  Bitmap Index Scan on de_kind_created_at_inc_dispute
		          (actual time=10.218..10.219 rows=29411 loops=1)
               Buffers: shared read=149
   (actual time=28.391..28.392 rows=17954 loops=1)
         Buckets: 32768  Batches: 1  Memory Usage: 1081kB
         Buffers: shared hit=1 read=111 using disputes_status_reason_id->  Hash 
         ->  Index Only Scan on disputes d
			          (actual time=0.242..17.394 rows=17954 loops=1)
               Heap Fetches: 0
               Buffers: shared hit=1 read=111
 Execution Time: 522.537 ms
```

The execution drops to **≈0.52 s**. The plan switches from a **Parallel Seq Scan** to a **Bitmap Index Scan → Bitmap Heap Scan**, so we’re no longer sweeping the whole table. Importantly, `Buffers: shared read` falls from **105k** to **~26k pages** (≈ **205 MiB**). That’s a big reduction, but we still touch many heap pages because we return `de.*`. The index range yields ~29k candidate rows, and only later the join with `disputes` narrows this down to **1,044**.

### **Partitioning (daily)**

I create a **daily time-series partitioned** table with `pg_partman`, move the data, and keep the same composite index

```sql
payments=# \d public.dispute_events_ts;
                   Partitioned table "public.dispute_events_ts"
      Column       |            Type             | Collation | Nullable | Default
-------------------+-----------------------------+-----------+----------+---------
 id                | character varying(255)      |           | not null |
 dispute_id        | character varying(255)      |           | not null |
 kind              | character varying(32)       |           | not null |
 provider_event_id | character varying(255)      |           | not null |
 data              | jsonb                       |           | not null |
 created_at        | timestamp without time zone |           | not null |
Partition key: RANGE (created_at)
Indexes:
    "dispute_events_pk" PRIMARY KEY, btree (id, created_at)
    "de_kind_created_at" btree (kind, created_at)
Foreign-key constraints:
    "fk_dispute_event_dispute" FOREIGN KEY (dispute_id) REFERENCES disputes(id)
Number of partitions: 93 (Use \d+ to list them.)
```

Same query on the partitioned table:

```sql
 Hash Join (actual time=14.027..64.403 rows=1044 loops=1)
   Hash Cond: ((de.dispute_id)::text = (d.id)::text)
   Buffers: shared hit=3831
   ->  Append (actual time=1.823..44.987 rows=29411 loops=1)
         Buffers: shared hit=3719
         ->  Bitmap Heap Scan on dispute_events_ts_p20250810 ...
           
         ->  Bitmap Heap Scan on dispute_events_ts_p20250811 ...
         
         ->  Bitmap Heap Scan on dispute_events_ts_p20250812 ...
               
         ->  Bitmap Heap Scan on dispute_events_ts_p20250813 ...
               
         ->  Bitmap Heap Scan on dispute_events_ts_p20250814 ...
               
   ->  Hash (actual time=11.419..11.420 rows=17954 loops=1)
         ->  Index Only Scan on disputes ...

Execution Time: 65.381 ms
```

### After partitioning

With daily partitions, the same 5-day query prunes to just **5 small child tables**. The number of candidate rows is still ~29k, but they are spread across small heaps and indexes. As a result the plan shows only **~3.8k buffer hits** (≈ **30 MiB**) instead of 26k reads. This means the working set fully fits in cache, so the query completes in **≈65 ms**. Partitioning doesn’t reduce the number of candidate rows, but it cuts the I/O needed to fetch them, which is why execution time drops from 0.52 s to ~0.06 s.