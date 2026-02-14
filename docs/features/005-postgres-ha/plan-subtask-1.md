# План: Streaming Replication Setup

## Мета
Піднати Docker Compose з PostgreSQL primary + standby (async streaming replication). Перевірити що дані реплікуються, standby доступний для read-запитів.

## Поточний стан
- Один PostgreSQL (`db` сервіс) з кастомним образом (pg_partman + `wal_level=logical`)
- `wal_level=logical` вже є — це суперсет `replica`, тому streaming replication працює без змін
- Init скрипт `scripts/init-db.sh` створює додаткову базу `ingest`

## Архітектурні рішення

| Питання | Рішення | Чому |
|---------|---------|------|
| Sync vs async replication | Async | Простіше, показує replication lag наочно |
| Як ініціалізувати standby | `pg_basebackup` в entrypoint скрипті | Стандартний підхід, не потребує зовнішніх інструментів |
| Окремий Docker image для replica | Ні, той самий `PG.Dockerfile` + інший entrypoint | Менше дублювання |
| Replication user | Окремий `replicator` юзер | Best practice — не давати replication права основному юзеру |
| Profile в compose | `replication` | Не ламає існуючий `infra` profile |

## Як працює streaming replication

1. **Primary** налаштовує:
   - `wal_level=logical` (вже є)
   - `max_wal_senders=3` (скільки standby можуть підключитися)
   - `wal_keep_size=64MB` (скільки WAL тримати для standby)
   - `hot_standby=on` (дозволити read-запити на standby)
   - Replication user в `pg_hba.conf`

2. **Standby** ініціалізується:
   - `pg_basebackup` — робить повну копію primary
   - Створює `standby.signal` файл — PG розуміє що це replica
   - `primary_conninfo` — connection string до primary
   - Після старту — отримує WAL stream і apply-ює зміни

## Імплементація

### 1. Модифікувати `PG.Dockerfile` — додати replication user

Додати до CMD параметри для replication:
```dockerfile
CMD ["postgres", \
     "-c", "shared_preload_libraries=pg_partman_bgw", \
     "-c", "wal_level=logical", \
     "-c", "max_wal_senders=3", \
     "-c", "wal_keep_size=64MB", \
     "-c", "hot_standby=on"]
```

### 2. Init скрипт для primary — створити replication user

Додати до `scripts/init-db.sh`:
```sql
CREATE ROLE replicator WITH REPLICATION LOGIN PASSWORD 'replicator_pass';
```

І дозволити replication в `pg_hba.conf`:
```bash
echo "host replication replicator all md5" >> "$PGDATA/pg_hba.conf"
```

### 3. Entrypoint скрипт для standby — `scripts/init-replica.sh`

```bash
#!/bin/bash
set -e

# Wait for primary
until pg_isready -h db-primary -U postgres; do
  echo "Waiting for primary..."
  sleep 1
done

# pg_basebackup — повна копія primary
pg_basebackup -h db-primary -U replicator -D /var/lib/postgresql/data -Fp -Xs -R -P

# -R flag створює standby.signal + primary_conninfo автоматично
# Після цього стандартний postgres entrypoint стартує як replica
exec docker-entrypoint.sh postgres \
  -c shared_preload_libraries=pg_partman_bgw \
  -c wal_level=logical \
  -c hot_standby=on
```

### 4. Docker Compose — додати `db-replica` сервіс

```yaml
db-primary:
  # Renamed from 'db', same config
  build:
    dockerfile: ./PG.Dockerfile
  profiles:
    - replication
  # ... existing db config with replication params

db-replica:
  build:
    dockerfile: ./PG.Dockerfile
  profiles:
    - replication
  depends_on:
    db-primary:
      condition: service_healthy
  environment:
    PGPASSWORD: replicator_pass
  entrypoint: ["/bin/bash", "/scripts/init-replica.sh"]
  volumes:
    - db-replica-data:/var/lib/postgresql/data/
    - ./scripts/init-replica.sh:/scripts/init-replica.sh
  ports:
    - '5433:5432'  # Different host port
```

### 5. Healthcheck на primary

```yaml
db-primary:
  healthcheck:
    test: ["CMD-SHELL", "pg_isready -U postgres"]
    interval: 5s
    timeout: 3s
    retries: 10
```

### 6. Перевірка

```bash
# Start replication setup
docker compose --profile replication up -d

# Check replication status on primary
docker exec db-primary psql -U postgres -c "SELECT * FROM pg_stat_replication;"

# Write on primary
docker exec db-primary psql -U postgres -d payments -c "SELECT count(*) FROM orders;"

# Read from replica (same data)
docker exec db-replica psql -U postgres -d payments -c "SELECT count(*) FROM orders;"

# Verify replica is read-only
docker exec db-replica psql -U postgres -d payments -c "CREATE TABLE test();"
# Expected: ERROR: cannot execute CREATE TABLE in a read-only transaction
```

## Файли для модифікації

| Файл | Зміни |
|------|-------|
| `PG.Dockerfile` | Додати `max_wal_senders`, `wal_keep_size`, `hot_standby` |
| `scripts/init-db.sh` | Створити replication user + pg_hba.conf entry |
| `scripts/init-replica.sh` | **Новий** — entrypoint для standby (pg_basebackup) |
| `docker-compose.yaml` | Додати `db-primary` + `db-replica` в profile `replication` |

## Порядок імплементації

1. Модифікувати `PG.Dockerfile` — додати replication параметри
2. Оновити `scripts/init-db.sh` — replication user + pg_hba
3. Створити `scripts/init-replica.sh` — standby entrypoint
4. Додати сервіси в `docker-compose.yaml` (profile `replication`)
5. Протестувати: `docker compose --profile replication up`
6. Перевірити replication status, read-only на replica
