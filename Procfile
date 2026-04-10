paymanager: set -a && source env/common.env && source env/endpoints.host.env && source env/paymanager.env && set +a && PORT=${API_PORT} WEBHOOK_MODE=kafka go run ./services/paymanager/cmd
ingest: set -a && source env/common.env && source env/endpoints.host.env && source env/ingest.env && set +a && PORT=${INGEST_PORT} WEBHOOK_MODE=kafka go run ./services/ingest/cmd
silvergate: set -a && source env/common.env && source env/endpoints.host.env && source env/silvergate.env && set +a && PORT=${SILVERGATE_PORT} go run ./services/silvergate/cmd
cdc: set -a && source env/common.env && source env/endpoints.host.env && source env/cdc.env && set +a && go run ./services/cdc/cmd
analytics: set -a && source env/common.env && source env/endpoints.host.env && source env/analytics.env && set +a && go run ./services/analytics/cmd
