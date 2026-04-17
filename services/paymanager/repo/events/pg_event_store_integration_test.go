//go:build integration

package events_test

import (
	"TestTaskJustPay/pkg/postgres"
	"TestTaskJustPay/services/paymanager/domain/events"
	events_repo "TestTaskJustPay/services/paymanager/repo/events"
	"context"
	"encoding/json"
	"errors"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestCreateEvent_Success(t *testing.T) {
	t.Parallel()
	ctx := context.Background()

	err := pool.SandboxTransaction(ctx, func(tx postgres.Executor) error {
		store := events_repo.NewPgEventStore(tx, pool.Builder)

		newEvent := events.NewEvent{
			AggregateType:  events.AggregateOrder,
			AggregateID:    "order_001",
			EventType:      "webhook_received",
			IdempotencyKey: "provider_evt_100",
			Payload:        json.RawMessage(`{"status":"created"}`),
			CreatedAt:      time.Date(2024, 1, 15, 10, 0, 0, 0, time.UTC),
		}

		created, err := store.CreateEvent(ctx, newEvent)
		require.NoError(t, err)
		require.NotNil(t, created)

		assert.NotEmpty(t, created.ID)
		assert.Equal(t, events.AggregateOrder, created.AggregateType)
		assert.Equal(t, "order_001", created.AggregateID)
		assert.Equal(t, "webhook_received", created.EventType)
		assert.Equal(t, "provider_evt_100", created.IdempotencyKey)
		assert.JSONEq(t, `{"status":"created"}`, string(created.Payload))
		assert.Equal(t, newEvent.CreatedAt, created.CreatedAt)

		return nil
	})
	require.NoError(t, err)
}

func TestCreateEvent_IdempotencyConstraint(t *testing.T) {
	t.Parallel()
	ctx := context.Background()

	tests := []struct {
		name                 string
		firstEvent           events.NewEvent
		duplicateEvent       events.NewEvent
		expectDuplicateError bool
	}{
		{
			name: "Duplicate (aggregate_type, aggregate_id, idempotency_key) returns ErrEventAlreadyStored",
			firstEvent: events.NewEvent{
				AggregateType:  events.AggregateOrder,
				AggregateID:    "order_001",
				EventType:      "webhook_received",
				IdempotencyKey: "provider_evt_123",
				Payload:        json.RawMessage(`{"status":"created"}`),
				CreatedAt:      time.Date(2024, 1, 15, 10, 0, 0, 0, time.UTC),
			},
			duplicateEvent: events.NewEvent{
				AggregateType:  events.AggregateOrder,
				AggregateID:    "order_001",
				EventType:      "webhook_received",
				IdempotencyKey: "provider_evt_123", // Same
				Payload:        json.RawMessage(`{"status":"updated"}`),
				CreatedAt:      time.Date(2024, 1, 15, 11, 0, 0, 0, time.UTC),
			},
			expectDuplicateError: true,
		},
		{
			name: "Same idempotency_key for different aggregates succeeds",
			firstEvent: events.NewEvent{
				AggregateType:  events.AggregateOrder,
				AggregateID:    "order_001",
				EventType:      "webhook_received",
				IdempotencyKey: "provider_evt_shared",
				Payload:        json.RawMessage(`{"status":"created"}`),
				CreatedAt:      time.Date(2024, 1, 15, 10, 0, 0, 0, time.UTC),
			},
			duplicateEvent: events.NewEvent{
				AggregateType:  events.AggregateOrder,
				AggregateID:    "order_002", // Different aggregate
				EventType:      "webhook_received",
				IdempotencyKey: "provider_evt_shared", // Same key
				Payload:        json.RawMessage(`{"status":"created"}`),
				CreatedAt:      time.Date(2024, 1, 15, 10, 0, 0, 0, time.UTC),
			},
			expectDuplicateError: false,
		},
		{
			name: "Same aggregate_id with different idempotency_key succeeds",
			firstEvent: events.NewEvent{
				AggregateType:  events.AggregateOrder,
				AggregateID:    "order_001",
				EventType:      "webhook_received",
				IdempotencyKey: "provider_evt_first",
				Payload:        json.RawMessage(`{"status":"created"}`),
				CreatedAt:      time.Date(2024, 1, 15, 10, 0, 0, 0, time.UTC),
			},
			duplicateEvent: events.NewEvent{
				AggregateType:  events.AggregateOrder,
				AggregateID:    "order_001",
				EventType:      "webhook_received",
				IdempotencyKey: "provider_evt_second", // Different key
				Payload:        json.RawMessage(`{"status":"updated"}`),
				CreatedAt:      time.Date(2024, 1, 15, 11, 0, 0, 0, time.UTC),
			},
			expectDuplicateError: false,
		},
		{
			name: "Same idempotency_key for different aggregate types succeeds",
			firstEvent: events.NewEvent{
				AggregateType:  events.AggregateOrder,
				AggregateID:    "entity_001",
				EventType:      "webhook_received",
				IdempotencyKey: "provider_evt_cross",
				Payload:        json.RawMessage(`{"status":"created"}`),
				CreatedAt:      time.Date(2024, 1, 15, 10, 0, 0, 0, time.UTC),
			},
			duplicateEvent: events.NewEvent{
				AggregateType:  events.AggregateDispute, // Different aggregate type
				AggregateID:    "entity_001",            // Same ID but different type
				EventType:      "chargeback_opened",
				IdempotencyKey: "provider_evt_cross", // Same key
				Payload:        json.RawMessage(`{"reason":"fraud"}`),
				CreatedAt:      time.Date(2024, 1, 15, 10, 0, 0, 0, time.UTC),
			},
			expectDuplicateError: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := pool.SandboxTransaction(ctx, func(tx postgres.Executor) error {
				store := events_repo.NewPgEventStore(tx, pool.Builder)

				// Create first event
				firstCreated, err := store.CreateEvent(ctx, tt.firstEvent)
				require.NoError(t, err)
				require.NotNil(t, firstCreated)
				assert.Equal(t, tt.firstEvent.AggregateID, firstCreated.AggregateID)
				assert.Equal(t, tt.firstEvent.IdempotencyKey, firstCreated.IdempotencyKey)

				// Attempt to create duplicate event
				duplicateCreated, err := store.CreateEvent(ctx, tt.duplicateEvent)

				if tt.expectDuplicateError {
					require.Error(t, err)
					assert.True(t, errors.Is(err, events.ErrEventAlreadyStored),
						"Expected ErrEventAlreadyStored, got: %v", err)
					assert.Nil(t, duplicateCreated)
				} else {
					require.NoError(t, err)
					require.NotNil(t, duplicateCreated)
					assert.Equal(t, tt.duplicateEvent.AggregateID, duplicateCreated.AggregateID)
					assert.Equal(t, tt.duplicateEvent.IdempotencyKey, duplicateCreated.IdempotencyKey)
				}

				return nil
			})

			require.NoError(t, err)
		})
	}
}
