-- ============ params ============
-- Підіграй під свій обсяг
-- 500k orders, 500k disputes, в середньому ~5 подій на диспут
DO $$
DECLARE
ORDERS_CNT              int := 500000;
  DISPUTES_CNT            int := 500000;
  MAX_EVENTS_PER_DISPUTE  int := 10;      -- буде 3..(3+MAX)
  DATE_FROM               timestamp := timestamp '2025-08-01';
  DATE_TO                 timestamp := timestamp '2025-09-01';
BEGIN
  -- Для генерації UUID
  PERFORM 1 FROM pg_extension WHERE extname='pgcrypto';
  IF NOT FOUND THEN
    EXECUTE 'CREATE EXTENSION IF NOT EXISTS pgcrypto';
END IF;

  -- Опційно: прискорити саме засівання (не залишай так для прод!)
EXECUTE 'SET LOCAL synchronous_commit = off';

-- ----------- (0) Очистка (ОБЕРЕЖНО!) -----------
-- Якщо хочеш перезасіяти з нуля, розкоментуй:
-- EXECUTE 'TRUNCATE dispute_events, disputes, orders RESTART IDENTITY CASCADE';

-- ----------- (1) orders -----------
INSERT INTO orders (id, user_id, status, created_at, updated_at)
SELECT
    'ord-' || lpad(g::text, 8, '0')                                       AS id,
    gen_random_uuid()                                                      AS user_id,
    (ARRAY['created','updated','failed','success'])[1+floor(random()*4)::int],
    ts                                                                     AS created_at,
    ts                                                                     AS updated_at
FROM (
    SELECT g,
    DATE_FROM + (random() * (DATE_TO - DATE_FROM)) AS ts
    FROM generate_series(1, ORDERS_CNT) AS g
    ) s;

-- ----------- (2) disputes -----------
INSERT INTO disputes (id, order_id, submitting_id, status, reason, amount, currency,
                      opened_at, evidence_due_at, submitted_at, closed_at)
SELECT
    'disp-' || lpad(g::text, 9, '0')                                       AS id,
    'ord-' || lpad((1 + floor(random()*ORDERS_CNT))::int::text, 8, '0')    AS order_id,
    CASE WHEN random() < 0.8 THEN 'sg-subm-'|| (1 + floor(random()*20))::int ELSE NULL END,
    (ARRAY['open','under_review','submitted','won','lost', 'closed', 'canceled'])[1+floor(random()*7)::int],
    (ARRAY['fraud','product_not_received','duplicate','other'])[1+floor(random()*4)::int],
    round((random()*500 + 5)::numeric, 2)                                   AS amount,
    (ARRAY['USD','EUR','UAH','GBP'])[1+floor(random()*4)::int]              AS currency,
    ts_open                                                                  AS opened_at,
    ts_open + (random()*20) * interval '1 day'                             AS evidence_due_at,
    CASE WHEN random() < 0.6 THEN ts_open + (random()*30) * interval '1 day' END AS submitted_at,
    CASE WHEN random() < 0.4 THEN ts_open + (random()*45) * interval '1 day' END AS closed_at
  FROM (
    SELECT g,
           DATE_FROM + (random() * (DATE_TO - DATE_FROM)) AS ts_open
    FROM generate_series(1, DISPUTES_CNT) AS g
  ) s;


  -- ----------- (3) dispute_events -----------
  -- 3..(3+MAX_EVENTS_PER_DISPUTE) подій на кожен диспут,
  -- created_at розкиданий у ~60 днів від opened_at
INSERT INTO dispute_events (id, dispute_id, kind, provider_event_id, data, created_at)
SELECT
    'de-' || lpad((row_number() over())::text, 12, '0')                     AS id,
    d.id                                                                     AS dispute_id,
    (ARRAY['webhook_opened','webhook_updated','provider_decision','evidence_submitted','evidence_added'])
    [1+floor(random()*5)::int]                                             AS kind,
    'prov-' || md5(g::text || d.id || clock_timestamp()::text || random()::text) AS provider_event_id,
    jsonb_build_object(
      'seq', g,
      'amount', round((random()*500)::numeric, 2),
      'note', 'seed'
    )                                                                         AS data,
    d.opened_at
      + make_interval(days   => (random()*60)::int,
                      hours  => (random()*24)::int,
                      mins   => (random()*60)::int)                           AS created_at
FROM disputes d
    JOIN LATERAL generate_series(1, 3 + floor(random()*MAX_EVENTS_PER_DISPUTE)::int) AS g ON true;

-- ----------- (5) статистика -----------
ANALYZE orders;
  ANALYZE disputes;
  ANALYZE dispute_events;
END$$;
