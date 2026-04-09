package analytics

import (
	"encoding/json"
	"time"
)

// event mirrors the JSON structure published by the CDC worker to Kafka.
// This is the contract between CDC and Analytics — no cross-service imports.
type event struct {
	ID             string          `json:"id"`
	AggregateType  string          `json:"aggregate_type"`
	AggregateID    string          `json:"aggregate_id"`
	EventType      string          `json:"event_type"`
	IdempotencyKey string          `json:"idempotency_key"`
	Payload        json.RawMessage `json:"payload"`
	CreatedAt      string          `json:"created_at"`
}

// pgTimestampFormat is the text representation PostgreSQL uses for
// timestamptz in logical replication output (e.g. "2026-02-14 11:51:37.01899+00").
const pgTimestampFormat = "2006-01-02 15:04:05.999999-07"

// normalizeTimestamp converts a PG-formatted timestamp to RFC 3339, which
// OpenSearch accepts under its default strict_date_optional_time format.
// If the value is already valid RFC 3339 or unparseable, it is returned as-is.
func normalizeTimestamp(raw string) string {
	t, err := time.Parse(pgTimestampFormat, raw)
	if err != nil {
		return raw // already ISO 8601 or something else — pass through
	}
	return t.Format(time.RFC3339Nano)
}
