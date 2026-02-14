#!/bin/bash
set -e

# Create additional databases needed by services.
# The default "payments" database is created by POSTGRES_DB env var.
psql -v ON_ERROR_STOP=1 --username "$POSTGRES_USER" --dbname "$POSTGRES_DB" <<-EOSQL
    CREATE DATABASE ingest;
EOSQL

# Create replication role for streaming replication (standby servers).
psql -v ON_ERROR_STOP=1 --username "$POSTGRES_USER" --dbname "$POSTGRES_DB" <<-EOSQL
    CREATE ROLE replicator WITH REPLICATION LOGIN PASSWORD 'replicator_pass';
EOSQL

# Allow replication connections from any host (Docker network).
echo "host replication replicator all md5" >> "$PGDATA/pg_hba.conf"
