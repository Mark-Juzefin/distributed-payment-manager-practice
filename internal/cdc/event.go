package cdc

import (
	"encoding/json"
	"fmt"

	"github.com/jackc/pglogrepl"
)

// walEvent represents a row from the `events` table decoded from WAL.
// Fields map 1:1 to table columns; payload is kept as raw JSON.
type walEvent struct {
	ID             string          `json:"id"`
	AggregateType  string          `json:"aggregate_type"`
	AggregateID    string          `json:"aggregate_id"`
	EventType      string          `json:"event_type"`
	IdempotencyKey string          `json:"idempotency_key"`
	Payload        json.RawMessage `json:"payload"`
	CreatedAt      string          `json:"created_at"`
}

// decodeInsert converts an InsertMessage into a walEvent using the column
// metadata from the RelationMessage. Returns an error if required columns
// are missing or the tuple cannot be decoded.
func decodeInsert(rel *pglogrepl.RelationMessage, msg *pglogrepl.InsertMessage) (*walEvent, error) {
	if msg.Tuple == nil {
		return nil, fmt.Errorf("InsertMessage has nil tuple")
	}

	// Build column name → text value map.
	values := make(map[string]string, rel.ColumnNum)
	for i, col := range msg.Tuple.Columns {
		if i >= int(rel.ColumnNum) {
			break
		}
		if col.DataType == 't' { // text representation
			values[rel.Columns[i].Name] = string(col.Data)
		}
	}

	evt := &walEvent{
		ID:             values["id"],
		AggregateType:  values["aggregate_type"],
		AggregateID:    values["aggregate_id"],
		EventType:      values["event_type"],
		IdempotencyKey: values["idempotency_key"],
		CreatedAt:      values["created_at"],
	}

	if raw, ok := values["payload"]; ok {
		evt.Payload = json.RawMessage(raw)
	}

	if evt.AggregateID == "" {
		return nil, fmt.Errorf("missing aggregate_id in WAL tuple")
	}

	return evt, nil
}
