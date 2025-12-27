//go:build integration
// +build integration

package order_eventsink_test

import (
	"TestTaskJustPay/internal/controller/apperror"
	"TestTaskJustPay/internal/domain/order"
	"TestTaskJustPay/internal/repo/order_eventsink"
	"TestTaskJustPay/pkg/postgres"
	"context"
	"errors"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestCreateOrderEvent_IdempotencyConstraint(t *testing.T) {
	t.Parallel()
	ctx := context.Background()

	tests := []struct {
		name                 string
		seed                 func(t *testing.T, tx postgres.Executor)
		firstEvent           order.NewOrderEvent
		duplicateEvent       order.NewOrderEvent
		expectDuplicateError bool
	}{
		{
			name: "Duplicate provider_event_id for same order returns ErrEventAlreadyStored",
			seed: func(t *testing.T, tx postgres.Executor) {
				applyBaseFixture(t, tx)
			},
			firstEvent: order.NewOrderEvent{
				OrderID:         "order_001",
				Kind:            order.OrderEventWebhookReceived,
				ProviderEventID: "provider_evt_123",
				Data:            []byte(`{"status": "created"}`),
				CreatedAt:       time.Date(2024, 1, 15, 10, 0, 0, 0, time.UTC),
			},
			duplicateEvent: order.NewOrderEvent{
				OrderID:         "order_001",
				Kind:            order.OrderEventWebhookReceived,
				ProviderEventID: "provider_evt_123",              // Same provider_event_id
				Data:            []byte(`{"status": "updated"}`), // Different data
				CreatedAt:       time.Date(2024, 1, 15, 11, 0, 0, 0, time.UTC),
			},
			expectDuplicateError: true,
		},
		{
			name: "Same provider_event_id for different orders succeeds",
			seed: func(t *testing.T, tx postgres.Executor) {
				applyBaseFixture(t, tx)
			},
			firstEvent: order.NewOrderEvent{
				OrderID:         "order_001",
				Kind:            order.OrderEventWebhookReceived,
				ProviderEventID: "provider_evt_shared",
				Data:            []byte(`{"status": "created"}`),
				CreatedAt:       time.Date(2024, 1, 15, 10, 0, 0, 0, time.UTC),
			},
			duplicateEvent: order.NewOrderEvent{
				OrderID:         "order_002", // Different order
				Kind:            order.OrderEventWebhookReceived,
				ProviderEventID: "provider_evt_shared", // Same provider_event_id (different orders can have same)
				Data:            []byte(`{"status": "created"}`),
				CreatedAt:       time.Date(2024, 1, 15, 11, 0, 0, 0, time.UTC),
			},
			expectDuplicateError: false,
		},
		{
			name: "Different provider_event_id for same order succeeds",
			seed: func(t *testing.T, tx postgres.Executor) {
				applyBaseFixture(t, tx)
			},
			firstEvent: order.NewOrderEvent{
				OrderID:         "order_001",
				Kind:            order.OrderEventWebhookReceived,
				ProviderEventID: "provider_evt_first",
				Data:            []byte(`{"status": "created"}`),
				CreatedAt:       time.Date(2024, 1, 15, 10, 0, 0, 0, time.UTC),
			},
			duplicateEvent: order.NewOrderEvent{
				OrderID:         "order_001",
				Kind:            order.OrderEventWebhookReceived,
				ProviderEventID: "provider_evt_second", // Different provider_event_id
				Data:            []byte(`{"status": "updated"}`),
				CreatedAt:       time.Date(2024, 1, 15, 11, 0, 0, 0, time.UTC),
			},
			expectDuplicateError: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := pool.SandboxTransaction(ctx, func(tx postgres.Executor) error {
				tt.seed(t, tx)

				repo := order_eventsink.NewPgOrderEventRepo(tx, pool.Builder)

				// Create first event
				firstCreated, err := repo.CreateOrderEvent(ctx, tt.firstEvent)
				require.NoError(t, err)
				require.NotNil(t, firstCreated)
				assert.Equal(t, tt.firstEvent.OrderID, firstCreated.OrderID)
				assert.Equal(t, tt.firstEvent.ProviderEventID, firstCreated.ProviderEventID)

				// Attempt to create duplicate event
				duplicateCreated, err := repo.CreateOrderEvent(ctx, tt.duplicateEvent)

				if tt.expectDuplicateError {
					// Should return ErrEventAlreadyStored for duplicate (order_id, provider_event_id)
					require.Error(t, err)
					assert.True(t, errors.Is(err, apperror.ErrEventAlreadyStored),
						"Expected ErrEventAlreadyStored, got: %v", err)
					assert.Nil(t, duplicateCreated)
				} else {
					// Should succeed for different combinations
					require.NoError(t, err)
					require.NotNil(t, duplicateCreated)
					assert.Equal(t, tt.duplicateEvent.OrderID, duplicateCreated.OrderID)
					assert.Equal(t, tt.duplicateEvent.ProviderEventID, duplicateCreated.ProviderEventID)
				}

				return nil
			})

			require.NoError(t, err)
		})
	}
}
