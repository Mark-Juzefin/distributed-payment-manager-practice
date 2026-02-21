#!/bin/bash
set -e

# Ensure Patroni data directory exists with correct permissions.
# Docker volume mounts /var/lib/postgresql/data as root-owned 1777,
# so we need to create the subdirectory with 0700 at runtime.
PGDATA=/var/lib/postgresql/data/patroni
if [ ! -d "$PGDATA" ]; then
    mkdir -p "$PGDATA"
fi
chmod 0700 "$PGDATA"

exec patroni "$@"
