package webhook

import (
	"TestTaskJustPay/pkg/messaging"
	"TestTaskJustPay/services/ingest/dto"
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// mockPublisher captures the last published envelope for assertions.
type mockPublisher struct {
	lastEnvelope messaging.Envelope
	publishErr   error
}

func (m *mockPublisher) Publish(_ context.Context, env messaging.Envelope) error {
	m.lastEnvelope = env
	return m.publishErr
}

func (m *mockPublisher) Close() error {
	return nil
}

func TestAsyncProcessor_PartitionKey(t *testing.T) {
	t.Run("ProcessOrderUpdate uses UserID as partition key", func(t *testing.T) {
		// Arrange
		mockPub := &mockPublisher{}
		processor := NewAsyncProcessor(mockPub, nil, nil)

		req := dto.OrderUpdateRequest{
			ProviderEventID: "evt-123",
			OrderID:         "order-AAA",
			UserID:          "user-BBB", // Different from OrderID!
			Status:          "created",
			CreatedAt:       time.Now(),
			UpdatedAt:       time.Now(),
		}

		// Act
		err := processor.ProcessOrderUpdate(context.Background(), req)

		// Assert
		require.NoError(t, err)
		// Key MUST be UserID for sharding-ready architecture
		assert.Equal(t, "user-BBB", mockPub.lastEnvelope.Key,
			"Partition key should be UserID, not OrderID")
	})

	t.Run("ProcessDisputeUpdate uses UserID as partition key", func(t *testing.T) {
		// Arrange
		mockPub := &mockPublisher{}
		processor := NewAsyncProcessor(nil, mockPub, nil)

		req := dto.DisputeUpdateRequest{
			ProviderEventID: "evt-456",
			OrderID:         "order-XXX",
			UserID:          "user-YYY", // Different from OrderID!
			Status:          "opened",
			Reason:          "fraud",
			OccurredAt:      time.Now(),
		}

		// Act
		err := processor.ProcessDisputeUpdate(context.Background(), req)

		// Assert
		require.NoError(t, err)
		// Key MUST be UserID for sharding-ready architecture
		assert.Equal(t, "user-YYY", mockPub.lastEnvelope.Key,
			"Partition key should be UserID, not OrderID")
	})
}
