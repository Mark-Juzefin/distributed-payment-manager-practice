#!/bin/bash
# Send a test order webhook via Ingest service (full flow)
# Usage: ./scripts/send-test-webhook.sh [status]
#   status: created, updated, success, failed (default: created)

# Load common env if not already exported
if [ -z "$INGEST_PORT" ]; then
    set -a
    source env/common.env 2>/dev/null || true
    set +a
fi

STATUS=${1:-created}
: ${INGEST_PORT:=3001}

# Generate random IDs
ORDER_ID="$(uuidgen)"
USER_ID="$(uuidgen)"
EVENT_ID="$(uuidgen)"
NOW=$(date -u +"%Y-%m-%dT%H:%M:%SZ")

URL="http://localhost:${INGEST_PORT}/webhooks/payments/orders"
echo "Sending webhook via Ingest service"

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
