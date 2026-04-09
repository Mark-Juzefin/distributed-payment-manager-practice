package webhook

import (
	"TestTaskJustPay/services/ingest/dto"
	"TestTaskJustPay/services/ingest/repo/inbox"
	"context"
	"errors"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type mockInboxRepo struct {
	lastMsg  inbox.NewInboxMessage
	storeErr error
}

func (m *mockInboxRepo) Store(_ context.Context, msg inbox.NewInboxMessage) error {
	m.lastMsg = msg
	return m.storeErr
}

func (m *mockInboxRepo) FetchPending(_ context.Context, _ int) ([]inbox.InboxMessage, error) {
	return nil, nil
}

func (m *mockInboxRepo) MarkProcessed(_ context.Context, _ string) error {
	return nil
}

func (m *mockInboxRepo) MarkFailed(_ context.Context, _ string, _ string, _ int) error {
	return nil
}

func TestInboxProcessor_ProcessOrderUpdate(t *testing.T) {
	t.Run("stores with correct idempotency key and webhook type", func(t *testing.T) {
		mock := &mockInboxRepo{}
		processor := NewInboxProcessor(mock)

		req := dto.OrderUpdateRequest{
			ProviderEventID: "evt-123",
			OrderID:         "order-AAA",
			UserID:          "user-BBB",
			Status:          "created",
			CreatedAt:       time.Date(2024, 1, 15, 10, 0, 0, 0, time.UTC),
			UpdatedAt:       time.Date(2024, 1, 15, 10, 0, 0, 0, time.UTC),
		}

		err := processor.ProcessOrderUpdate(context.Background(), req)

		require.NoError(t, err)
		assert.Equal(t, "order_update:evt-123", mock.lastMsg.IdempotencyKey)
		assert.Equal(t, "order_update", mock.lastMsg.WebhookType)
		assert.NotEmpty(t, mock.lastMsg.Payload)
	})

	t.Run("swallows ErrAlreadyExists", func(t *testing.T) {
		mock := &mockInboxRepo{storeErr: inbox.ErrAlreadyExists}
		processor := NewInboxProcessor(mock)

		req := dto.OrderUpdateRequest{
			ProviderEventID: "evt-duplicate",
			OrderID:         "order-AAA",
			UserID:          "user-BBB",
			Status:          "created",
			CreatedAt:       time.Now(),
			UpdatedAt:       time.Now(),
		}

		err := processor.ProcessOrderUpdate(context.Background(), req)
		assert.NoError(t, err)
	})

	t.Run("propagates other errors", func(t *testing.T) {
		mock := &mockInboxRepo{storeErr: errors.New("connection refused")}
		processor := NewInboxProcessor(mock)

		req := dto.OrderUpdateRequest{
			ProviderEventID: "evt-123",
			OrderID:         "order-AAA",
			UserID:          "user-BBB",
			Status:          "created",
			CreatedAt:       time.Now(),
			UpdatedAt:       time.Now(),
		}

		err := processor.ProcessOrderUpdate(context.Background(), req)
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "connection refused")
	})
}

func TestInboxProcessor_ProcessDisputeUpdate(t *testing.T) {
	t.Run("stores with correct idempotency key and webhook type", func(t *testing.T) {
		mock := &mockInboxRepo{}
		processor := NewInboxProcessor(mock)

		req := dto.DisputeUpdateRequest{
			ProviderEventID: "evt-456",
			OrderID:         "order-XXX",
			UserID:          "user-YYY",
			Status:          "opened",
			Reason:          "fraud",
			OccurredAt:      time.Date(2024, 1, 15, 10, 0, 0, 0, time.UTC),
		}

		err := processor.ProcessDisputeUpdate(context.Background(), req)

		require.NoError(t, err)
		assert.Equal(t, "dispute_update:evt-456", mock.lastMsg.IdempotencyKey)
		assert.Equal(t, "dispute_update", mock.lastMsg.WebhookType)
		assert.NotEmpty(t, mock.lastMsg.Payload)
	})

	t.Run("swallows ErrAlreadyExists", func(t *testing.T) {
		mock := &mockInboxRepo{storeErr: inbox.ErrAlreadyExists}
		processor := NewInboxProcessor(mock)

		req := dto.DisputeUpdateRequest{
			ProviderEventID: "evt-duplicate",
			OrderID:         "order-XXX",
			UserID:          "user-YYY",
			Status:          "opened",
			Reason:          "fraud",
			OccurredAt:      time.Now(),
		}

		err := processor.ProcessDisputeUpdate(context.Background(), req)
		assert.NoError(t, err)
	})
}
