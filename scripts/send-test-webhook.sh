#!/bin/bash
# Send a test order webhook
# Usage: ./scripts/send-test-webhook.sh [mode] [status]
#   mode: "api" (direct to API /internal/updates/orders) or "ingest" (via Ingest /webhooks/...)
#   status: created, updated, success, failed
# Default: mode=api, status=created

MODE=${1:-api}
STATUS=${2:-created}

# Generate random IDs
ORDER_ID="$(uuidgen)"
USER_ID="$(uuidgen)"
EVENT_ID="$(uuidgen)"
NOW=$(date -u +"%Y-%m-%dT%H:%M:%SZ")

if [ "$MODE" = "ingest" ]; then
    URL="http://localhost:3001/webhooks/payments/orders"
    echo "Sending webhook via Ingest service (external endpoint)"
else
    URL="http://localhost:3000/internal/updates/orders"
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
