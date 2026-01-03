package webhook

import (
	"TestTaskJustPay/internal/shared/domain/dispute"
	"TestTaskJustPay/internal/shared/domain/order"
	"TestTaskJustPay/internal/shared/messaging"
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
	t.Run("ProcessOrderWebhook uses UserId as partition key", func(t *testing.T) {
		// Arrange
		mockPub := &mockPublisher{}
		processor := NewAsyncProcessor(mockPub, nil)

		webhook := order.PaymentWebhook{
			ProviderEventID: "evt-123",
			OrderId:         "order-AAA",
			UserId:          "user-BBB", // Different from OrderId!
			Status:          order.StatusCreated,
			CreatedAt:       time.Now(),
			UpdatedAt:       time.Now(),
		}

		// Act
		err := processor.ProcessOrderWebhook(context.Background(), webhook)

		// Assert
		require.NoError(t, err)
		// Key MUST be UserId for sharding-ready architecture
		assert.Equal(t, "user-BBB", mockPub.lastEnvelope.Key,
			"Partition key should be UserId, not OrderId")
	})

	t.Run("ProcessDisputeWebhook uses UserID as partition key", func(t *testing.T) {
		// Arrange
		mockPub := &mockPublisher{}
		processor := NewAsyncProcessor(nil, mockPub)

		webhook := dispute.ChargebackWebhook{
			ProviderEventID: "evt-456",
			OrderID:         "order-XXX",
			UserID:          "user-YYY", // Different from OrderID!
			Status:          dispute.ChargebackOpened,
			Reason:          "fraud",
			OccurredAt:      time.Now(),
		}

		// Act
		err := processor.ProcessDisputeWebhook(context.Background(), webhook)

		// Assert
		require.NoError(t, err)
		// Key MUST be UserID for sharding-ready architecture
		assert.Equal(t, "user-YYY", mockPub.lastEnvelope.Key,
			"Partition key should be UserID, not OrderID")
	})
}
