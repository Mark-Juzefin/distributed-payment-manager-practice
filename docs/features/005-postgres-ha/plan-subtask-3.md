# План: Failover/Switchover — Patroni + etcd

## Мета

Замінити ручну streaming replication на Patroni — кластер-менеджер, який автоматично:
- Ініціалізує primary і репліки (замість init-db.sh / init-replica.sh)
- Керує pg_hba.conf, replication user, replication slots
- Робить автоматичний failover при падінні primary
- Надає REST API для HAProxy health checks

## Поточний стан

- Primary + 2 репліки, налаштовані вручну через shell-скрипти
- HAProxy роутить rw/ro трафік з `pgsql-check` health check
- Конфігурація розкидана: Dockerfile CMD, docker-compose command, init-db.sh, init-replica.sh
- Немає автоматичного failover — якщо primary впаде, все зупиняється

## Архітектурні рішення

| Питання | Рішення | Чому |
|---------|---------|------|
| DCS (distributed config store) | etcd, 1 нода | Для sandbox достатньо, в проді було б 3 |
| Patroni image | Розширити PG.Dockerfile: postgres:17 + pg_partman + patroni (pip) | Один образ для всіх нод, Patroni сам визначає роль |
| Конфігурація PG | Через patroni.yml (`postgresql.parameters`) | Єдине місце для всіх PG параметрів, Patroni пропагує на репліки |
| pg_hba.conf | Через patroni.yml (`postgresql.pg_hba`) | Patroni генерує автоматично, замість echo в init-db.sh |
| Replication user | Через patroni.yml (`authentication.replication`) | Patroni створює при bootstrap, замість CREATE ROLE в скрипті |
| HAProxy health check | Patroni REST API `:8008` (`/primary`, `/replica`) | Знає хто реально leader після failover, замість pgsql-check |
| Кількість нод PG | 3 (1 leader + 2 replica) | Як зараз, але всі однакові контейнери |
| Replication slots | Patroni керує автоматично | Замість відсутності слотів зараз |

## Цільова архітектура

```
                    ┌─────────┐
                    │  etcd   │  ← хто leader?
                    └────┬────┘
                         │
          ┌──────────────┼──────────────┐
          │              │              │
    ┌─────┴─────┐  ┌─────┴─────┐  ┌─────┴─────┐
    │ patroni1  │  │ patroni2  │  │ patroni3  │
    │ PG + Pat  │  │ PG + Pat  │  │ PG + Pat  │
    │ (leader)  │  │ (replica) │  │ (replica) │
    │ :5432     │  │ :5432     │  │ :5432     │
    │ :8008 API │  │ :8008 API │  │ :8008 API │
    └───────────┘  └───────────┘  └───────────┘
          │              │              │
          └──────────────┼──────────────┘
                         │
                    ┌────┴────┐
                    │ HAProxy │
                    │ :5440rw │  ← httpchk GET /primary
                    │ :5441ro │  ← httpchk GET /replica
                    └─────────┘
```

## Структура файлів

### Нові файли
```
config/patroni.yml          ← єдиний конфіг для всіх Patroni нод (параметризується через env vars)
```

### Модифіковані файли
```
PG.Dockerfile               ← додати: pip install patroni[etcd], прибрати CMD
docker-compose.yaml         ← замінити db-primary/db-replica/db-replica-2 на patroni1/2/3 + etcd
config/haproxy.cfg          ← змінити health check на Patroni REST API
```

### Файли що видаляються
```
scripts/init-db.sh          ← Patroni сам створює replication user і pg_hba
scripts/init-replica.sh     ← Patroni сам робить pg_basebackup
```

## Імплементація

### Крок 1: Оновити PG.Dockerfile

Додати Patroni поверх існуючого образу з pg_partman:

```dockerfile
FROM postgres:17

# pg_partman extension (як зараз)
RUN apt-get update && apt-get install -y --no-install-recommends \
      build-essential wget ca-certificates postgresql-server-dev-17
RUN wget "https://github.com/pgpartman/pg_partman/archive/refs/tags/v5.2.4.tar.gz" \
    && tar zxf v5.2.4.tar.gz && cd pg_partman-5.2.4 \
    && make && make install \
    && rm -rf /var/lib/apt/lists/* v5.2.4.tar.gz pg_partman-5.2.4

# Patroni + etcd client
RUN apt-get update && apt-get install -y --no-install-recommends \
      python3 python3-pip python3-dev libpq-dev \
    && pip3 install --break-system-packages patroni[etcd3] psycopg2-binary \
    && rm -rf /var/lib/apt/lists/*

# Patroni config
COPY config/patroni.yml /etc/patroni/patroni.yml

ENTRYPOINT ["patroni"]
CMD ["/etc/patroni/patroni.yml"]
```

Ключова зміна: **entrypoint = patroni**, не postgres. Patroni сам запускає і керує postgres процесом.

### Крок 2: Створити config/patroni.yml

```yaml
scope: payments-cluster
name: ${PATRONI_NAME}

etcd3:
  hosts: etcd:2379

restapi:
  listen: 0.0.0.0:8008
  connect_address: ${PATRONI_NAME}:8008

bootstrap:
  dcs:
    ttl: 30
    loop_wait: 10
    retry_timeout: 10
    maximum_lag_on_failover: 1048576  # 1MB — don't promote replica that's too far behind
    postgresql:
      use_pg_rewind: true
      use_slots: true
      parameters:
        shared_preload_libraries: 'pg_partman_bgw'
        wal_level: logical
        max_wal_senders: 10
        max_replication_slots: 10
        wal_keep_size: 256MB
        hot_standby: 'on'
  initdb:
    - encoding: UTF8
    - locale: en_US.UTF-8
    - data-checksums
  pg_hba:
    - host all all 0.0.0.0/0 md5
    - host replication replicator 0.0.0.0/0 md5

postgresql:
  listen: 0.0.0.0:5432
  connect_address: ${PATRONI_NAME}:5432
  data_dir: /var/lib/postgresql/data
  authentication:
    superuser:
      username: postgres
      password: secret
    replication:
      username: replicator
      password: replicator_pass
  parameters:
    shared_preload_libraries: 'pg_partman_bgw'
  pg_hba:
    - host all all 0.0.0.0/0 md5
    - host replication replicator 0.0.0.0/0 md5
```

Примітки:
- `scope` — ім'я кластера, однакове для всіх нод
- `name` — унікальне ім'я ноди, приходить з env var `PATRONI_NAME`
- `bootstrap.dcs` — параметри що зберігаються в etcd і шаряться між нодами
- `postgresql.parameters` — локальні параметри ноди (Patroni мерджить з DCS)
- `use_pg_rewind: true` — дозволяє старому primary повернутись як replica без повного basebackup
- `use_slots: true` — Patroni автоматично створює replication slots
- `data-checksums` — дозволяє pg_rewind працювати

### Крок 3: Оновити docker-compose.yaml

Замінити `db-primary`, `db-replica`, `db-replica-2` на однакові Patroni контейнери:

```yaml
  etcd:
    image: quay.io/coreos/etcd:v3.5.17
    container_name: etcd
    hostname: etcd
    profiles:
      - infra
      - prod
    networks:
      - backend
    environment:
      ETCD_LISTEN_CLIENT_URLS: http://0.0.0.0:2379
      ETCD_ADVERTISE_CLIENT_URLS: http://etcd:2379
      ETCD_LISTEN_PEER_URLS: http://0.0.0.0:2380
      ETCD_INITIAL_ADVERTISE_PEER_URLS: http://etcd:2380
      ETCD_INITIAL_CLUSTER: etcd=http://etcd:2380
      ETCD_INITIAL_CLUSTER_STATE: new
      ETCD_INITIAL_CLUSTER_TOKEN: payments-cluster
      ETCD_UNSUPPORTED_ARCH: arm64
    healthcheck:
      test: ["CMD", "etcdctl", "endpoint", "health"]
      interval: 5s
      timeout: 3s
      retries: 5

  patroni1:
    build:
      dockerfile: ./PG.Dockerfile
    container_name: patroni1
    hostname: patroni1
    profiles:
      - infra
      - prod
    networks:
      - backend
    environment:
      PATRONI_NAME: patroni1
    ports:
      - '5432:5432'
      - '8008:8008'
    volumes:
      - patroni1-data:/var/lib/postgresql/data
    depends_on:
      etcd:
        condition: service_healthy

  patroni2:
    build:
      dockerfile: ./PG.Dockerfile
    container_name: patroni2
    hostname: patroni2
    profiles:
      - infra
    networks:
      - backend
    environment:
      PATRONI_NAME: patroni2
    ports:
      - '5433:5432'
      - '8009:8008'
    volumes:
      - patroni2-data:/var/lib/postgresql/data
    depends_on:
      etcd:
        condition: service_healthy

  patroni3:
    build:
      dockerfile: ./PG.Dockerfile
    container_name: patroni3
    hostname: patroni3
    profiles:
      - infra
    networks:
      - backend
    environment:
      PATRONI_NAME: patroni3
    ports:
      - '5434:5432'
      - '8010:8008'
    volumes:
      - patroni3-data:/var/lib/postgresql/data
    depends_on:
      etcd:
        condition: service_healthy
```

### Крок 4: Оновити config/haproxy.cfg

Змінити health check з pgsql-check на Patroni REST API:

```
backend pg_primary
    mode tcp
    option httpchk GET /primary
    http-check expect status 200
    default-server inter 3s fall 3 rise 2 on-marked-down shutdown-sessions
    server patroni1 patroni1:5432 check port 8008
    server patroni2 patroni2:5432 check port 8008
    server patroni3 patroni3:5432 check port 8008

backend pg_replicas
    mode tcp
    balance roundrobin
    option httpchk GET /replica
    http-check expect status 200
    default-server inter 3s fall 3 rise 2
    server patroni1 patroni1:5432 check port 8008
    server patroni2 patroni2:5432 check port 8008
    server patroni3 patroni3:5432 check port 8008
```

Ключові зміни:
- `option httpchk` замість `option pgsql-check` — HAProxy робить HTTP запит до Patroni API
- Всі 3 ноди в обох backend-ах — HAProxy сам визначає хто primary, хто replica
- `on-marked-down shutdown-sessions` — при failover HAProxy одразу розриває з'єднання до старого primary
- `check port 8008` — health check йде на Patroni REST API, трафік на :5432

### Крок 5: Оновити env файли

```env
# endpoints.host.env
PG_URL=postgres://postgres:secret@localhost:5440/payments?sslmode=disable
PG_REPLICA_URL=postgres://postgres:secret@localhost:5441/payments?sslmode=disable

# endpoints.docker.env
PG_URL=postgres://postgres:secret@haproxy:5440/payments?sslmode=disable
PG_REPLICA_URL=postgres://postgres:secret@haproxy:5441/payments?sslmode=disable
```

Аплікація не змінюється — вона як і раніше підключається до HAProxy, який тепер знає хто leader.

### Крок 6: Оновити postgres-exporter

Замінити `db-primary` на `haproxy:5440` (завжди вказує на поточного leader):

```yaml
  postgres-exporter:
    environment:
      DATA_SOURCE_URI: haproxy:5440/payments?sslmode=disable
```

### Крок 7: Створити базу `payments`

Patroni за замовчуванням створює БД `postgres`. Для створення `payments` і `ingest` — додати post_bootstrap script або SQL в bootstrap секцію patroni.yml:

```yaml
bootstrap:
  post_bootstrap: /etc/patroni/post-bootstrap.sh
```

```bash
#!/bin/bash
psql -U postgres -c "CREATE DATABASE payments;"
psql -U postgres -c "CREATE DATABASE ingest;"
psql -U postgres -d payments -c "CREATE EXTENSION IF NOT EXISTS pg_partman;"
```

Або через `post_init` в patroni.yml.

### Крок 8: Оновити Makefile

```makefile
db-primary:
	PGPASSWORD=secret psql -h localhost -p 5440 -U postgres -d payments

db-replica:
	PGPASSWORD=secret psql -h localhost -p 5441 -U postgres -d payments

# Patroni cluster status
patroni-status:
	docker exec patroni1 patronictl -c /etc/patroni/patroni.yml list
```

### Крок 9: Видалити старі файли

- `scripts/init-db.sh` — Patroni bootstrap замінює
- `scripts/init-replica.sh` — Patroni сам робить pg_basebackup

### Крок 10: Оновити Grafana dashboard

Додати панелі:
- Patroni cluster state (leader/replica/unknown) — через Patroni REST API або etcd
- Failover counter

## Порядок імплементації

1. Оновити PG.Dockerfile (додати Patroni)
2. Створити config/patroni.yml
3. Створити post-bootstrap скрипт (CREATE DATABASE payments, ingest, pg_partman extension)
4. Оновити docker-compose.yaml (etcd + patroni1/2/3, видалити db-primary/replica/replica-2)
5. Оновити config/haproxy.cfg (Patroni REST API health checks)
6. Оновити env файли
7. Оновити postgres-exporter
8. Оновити Makefile
9. Видалити scripts/init-db.sh, scripts/init-replica.sh
10. Протестувати: запуск кластера, failover, switchover
11. Оновити Grafana dashboard

## Як тестувати failover

```bash
# Перевірити стан кластера
docker exec patroni1 patronictl -c /etc/patroni/patroni.yml list

# Ручний switchover (graceful)
docker exec patroni1 patronictl -c /etc/patroni/patroni.yml switchover

# Симулювати crash primary
docker stop patroni1

# Перевірити що HAProxy переключився
curl http://localhost:8404/stats
```
