#!/bin/bash
# Send a test order webhook
# Usage: ./scripts/send-test-webhook.sh [mode] [status]
#   mode: "api" (direct to API /internal/updates/orders) or "ingest" (via Ingest /webhooks/...)
#   status: created, updated, success, failed
# Default: mode=api, status=created
#
# Ports are read from env/common.env (or fallback to defaults)

# Load common env if not already exported
if [ -z "$API_PORT" ]; then
    set -a
    source env/common.env 2>/dev/null || true
    set +a
fi

MODE=${1:-api}
STATUS=${2:-created}

# Use env vars with defaults
: ${API_PORT:=3000}
: ${INGEST_PORT:=3001}

# Generate random IDs
ORDER_ID="$(uuidgen)"
USER_ID="$(uuidgen)"
EVENT_ID="$(uuidgen)"
NOW=$(date -u +"%Y-%m-%dT%H:%M:%SZ")

if [ "$MODE" = "ingest" ]; then
    URL="http://localhost:${INGEST_PORT}/webhooks/payments/orders"
    echo "Sending webhook via Ingest service (external endpoint)"
else
    URL="http://localhost:${API_PORT}/internal/updates/orders"
    echo "Sending webhook directly to API (internal endpoint)"
fi

echo "  url: $URL"
echo "  order_id: $ORDER_ID"
echo "  status: $STATUS"
echo ""

curl -s -X POST "$URL" \
  -H "Content-Type: application/json" \
  -d "{
    \"provider_event_id\": \"$EVENT_ID\",
    \"order_id\": \"$ORDER_ID\",
    \"user_id\": \"$USER_ID\",
    \"status\": \"$STATUS\",
    \"updated_at\": \"$NOW\",
    \"created_at\": \"$NOW\",
    \"meta\": {\"source\": \"test-script\"}
  }" | jq . 2>/dev/null || cat

echo ""
