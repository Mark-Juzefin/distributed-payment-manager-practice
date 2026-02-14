#!/bin/bash
set -e

echo "Waiting for primary to be ready..."
until pg_isready -h db-primary -U postgres; do
    echo "Primary not ready yet, retrying in 2s..."
    sleep 2
done

# Ensure PGDATA directory exists and is owned by postgres
mkdir -p "$PGDATA"
chown -R postgres:postgres "$PGDATA"
chmod 0700 "$PGDATA"

echo "Primary is ready. Running pg_basebackup..."
gosu postgres pg_basebackup -h db-primary -U replicator -D "$PGDATA" -Fp -Xs -R -P

echo "Base backup complete. Starting replica..."
exec gosu postgres postgres \
    -c "shared_preload_libraries=pg_partman_bgw" \
    -c "wal_level=logical" \
    -c "hot_standby=on"
