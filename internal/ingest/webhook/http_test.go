//go:build !integration

package webhook

import (
	"context"
	"errors"
	"testing"
	"time"

	"TestTaskJustPay/internal/api/domain/dispute"
	"TestTaskJustPay/internal/api/domain/order"
	"TestTaskJustPay/internal/shared/dto"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// mockClient is a mock implementation of apiclient.Client for testing.
type mockClient struct {
	lastOrderReq   dto.OrderUpdateRequest
	lastDisputeReq dto.DisputeUpdateRequest
	orderErr       error
	disputeErr     error
}

func (m *mockClient) SendOrderUpdate(_ context.Context, req dto.OrderUpdateRequest) error {
	m.lastOrderReq = req
	return m.orderErr
}

func (m *mockClient) SendDisputeUpdate(_ context.Context, req dto.DisputeUpdateRequest) error {
	m.lastDisputeReq = req
	return m.disputeErr
}

func (m *mockClient) Close() error {
	return nil
}

func TestHTTPSyncProcessor_ProcessOrderWebhook(t *testing.T) {
	t.Run("converts webhook to DTO and sends", func(t *testing.T) {
		mock := &mockClient{}
		processor := NewHTTPSyncProcessor(mock)

		now := time.Now()
		webhook := order.PaymentWebhook{
			ProviderEventID: "evt-123",
			OrderId:         "order-AAA",
			UserId:          "user-BBB",
			Status:          order.StatusCreated,
			CreatedAt:       now,
			UpdatedAt:       now,
			Meta: map[string]string{
				"amount": "100.50",
			},
		}

		err := processor.ProcessOrderWebhook(context.Background(), webhook)

		require.NoError(t, err)
		assert.Equal(t, "evt-123", mock.lastOrderReq.ProviderEventID)
		assert.Equal(t, "order-AAA", mock.lastOrderReq.OrderID)
		assert.Equal(t, "user-BBB", mock.lastOrderReq.UserID)
		assert.Equal(t, "created", mock.lastOrderReq.Status)
		assert.Equal(t, now, mock.lastOrderReq.CreatedAt)
		assert.Equal(t, "100.50", mock.lastOrderReq.Meta["amount"])
	})

	t.Run("propagates client errors", func(t *testing.T) {
		expectedErr := errors.New("connection failed")
		mock := &mockClient{orderErr: expectedErr}
		processor := NewHTTPSyncProcessor(mock)

		webhook := order.PaymentWebhook{
			ProviderEventID: "evt-123",
			OrderId:         "order-AAA",
			UserId:          "user-BBB",
			Status:          order.StatusCreated,
		}

		err := processor.ProcessOrderWebhook(context.Background(), webhook)

		assert.ErrorIs(t, err, expectedErr)
	})
}

func TestHTTPSyncProcessor_ProcessDisputeWebhook(t *testing.T) {
	t.Run("converts webhook to DTO and sends", func(t *testing.T) {
		mock := &mockClient{}
		processor := NewHTTPSyncProcessor(mock)

		now := time.Now()
		dueAt := now.Add(24 * time.Hour)
		webhook := dispute.ChargebackWebhook{
			ProviderEventID: "evt-456",
			OrderID:         "order-XXX",
			UserID:          "user-YYY",
			Status:          dispute.ChargebackOpened,
			Reason:          "fraud",
			Money: dispute.Money{
				Amount:   250.75,
				Currency: "USD",
			},
			OccurredAt:    now,
			EvidenceDueAt: &dueAt,
			Meta: map[string]string{
				"resolution": "pending",
			},
		}

		err := processor.ProcessDisputeWebhook(context.Background(), webhook)

		require.NoError(t, err)
		assert.Equal(t, "evt-456", mock.lastDisputeReq.ProviderEventID)
		assert.Equal(t, "order-XXX", mock.lastDisputeReq.OrderID)
		assert.Equal(t, "user-YYY", mock.lastDisputeReq.UserID)
		assert.Equal(t, "opened", mock.lastDisputeReq.Status)
		assert.Equal(t, "fraud", mock.lastDisputeReq.Reason)
		assert.Equal(t, 250.75, mock.lastDisputeReq.Amount)
		assert.Equal(t, "USD", mock.lastDisputeReq.Currency)
		assert.Equal(t, now, mock.lastDisputeReq.OccurredAt)
		assert.Equal(t, &dueAt, mock.lastDisputeReq.EvidenceDueAt)
	})

	t.Run("propagates client errors", func(t *testing.T) {
		expectedErr := errors.New("timeout")
		mock := &mockClient{disputeErr: expectedErr}
		processor := NewHTTPSyncProcessor(mock)

		webhook := dispute.ChargebackWebhook{
			ProviderEventID: "evt-456",
			OrderID:         "order-XXX",
			UserID:          "user-YYY",
			Status:          dispute.ChargebackOpened,
		}

		err := processor.ProcessDisputeWebhook(context.Background(), webhook)

		assert.ErrorIs(t, err, expectedErr)
	})
}
