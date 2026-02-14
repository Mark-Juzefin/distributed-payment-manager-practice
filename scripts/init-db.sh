#!/bin/bash
set -e

# Create additional databases needed by services.
# The default "payments" database is created by POSTGRES_DB env var.
psql -v ON_ERROR_STOP=1 --username "$POSTGRES_USER" --dbname "$POSTGRES_DB" <<-EOSQL
    CREATE DATABASE ingest;
EOSQL
