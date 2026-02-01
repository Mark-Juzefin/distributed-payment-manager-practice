api: set -a && source env/common.env && source env/endpoints.host.env && source env/api.env && set +a && PORT=${API_PORT} WEBHOOK_MODE=kafka go run ./cmd/api
ingest: set -a && source env/common.env && source env/endpoints.host.env && source env/ingest.env && set +a && PORT=${INGEST_PORT} WEBHOOK_MODE=kafka go run ./cmd/ingest
