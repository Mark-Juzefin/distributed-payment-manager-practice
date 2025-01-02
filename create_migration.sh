#!/bin/sh

MIGRATION_NAME="$1"
if [ -z "$MIGRATION_NAME" ]; then help; exit; fi

goose -dir=src/app/migration create "${MIGRATION_NAME}" sql