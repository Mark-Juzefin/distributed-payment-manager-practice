#!/bin/bash
set -e

echo "Post-bootstrap: creating databases..."

psql -U postgres -c "CREATE DATABASE payments;"
psql -U postgres -c "CREATE DATABASE ingest;"

echo "Post-bootstrap complete."
