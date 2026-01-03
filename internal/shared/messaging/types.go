package messaging

import (
	"context"
	"encoding/json"
	"time"

	"github.com/google/uuid"
)

// Envelope wraps a message with metadata for tracing and routing.
type Envelope struct {
	EventID   string          `json:"event_id"`
	Key       string          `json:"key"`
	Type      string          `json:"type"`
	Payload   json.RawMessage `json:"payload"`
	Timestamp time.Time       `json:"timestamp"`
}

// NewEnvelope creates a new envelope with a generated event ID.
func NewEnvelope(key, msgType string, payload any) (Envelope, error) {
	data, err := json.Marshal(payload)
	if err != nil {
		return Envelope{}, err
	}

	return Envelope{
		EventID:   uuid.New().String(),
		Key:       key,
		Type:      msgType,
		Payload:   data,
		Timestamp: time.Now().UTC(),
	}, nil
}

// Publisher sends messages to a message broker.
type Publisher interface {
	Publish(ctx context.Context, envelope Envelope) error
	Close() error
}

// MessageHandler processes a single message.
type MessageHandler func(ctx context.Context, key, value []byte) error

// Worker consumes messages from a message broker.
type Worker interface {
	Start(ctx context.Context, handler MessageHandler) error
	Close() error
}
