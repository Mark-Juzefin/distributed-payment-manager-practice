package worker

import (
	"context"
	"encoding/json"
	"fmt"
	"testing"
	"time"

	"TestTaskJustPay/services/ingest/apiclient"
	"TestTaskJustPay/services/ingest/dto"
	"TestTaskJustPay/services/ingest/repo/inbox"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/mock/gomock"
)

func newTestWorker(repo inbox.InboxRepo, client apiclient.Client) *InboxWorker {
	return NewInboxWorker(repo, client, Config{
		PollInterval: 50 * time.Millisecond,
		BatchSize:    10,
		MaxRetries:   3,
	})
}

func orderPayload(t *testing.T) json.RawMessage {
	t.Helper()
	req := dto.OrderUpdateRequest{
		ProviderEventID: "evt_001",
		OrderID:         "order_001",
		UserID:          "user_001",
		Status:          "created",
		UpdatedAt:       time.Date(2024, 1, 15, 10, 0, 0, 0, time.UTC),
		CreatedAt:       time.Date(2024, 1, 15, 10, 0, 0, 0, time.UTC),
	}
	b, err := json.Marshal(req)
	require.NoError(t, err)
	return b
}

func disputePayload(t *testing.T) json.RawMessage {
	t.Helper()
	req := dto.DisputeUpdateRequest{
		ProviderEventID: "evt_002",
		OrderID:         "order_001",
		UserID:          "user_001",
		Status:          "open",
		Reason:          "fraud",
		Amount:          100.0,
		Currency:        "USD",
		OccurredAt:      time.Date(2024, 1, 15, 10, 0, 0, 0, time.UTC),
	}
	b, err := json.Marshal(req)
	require.NoError(t, err)
	return b
}

func TestProcessMessage_OrderUpdate_Success(t *testing.T) {
	ctrl := gomock.NewController(t)
	mockRepo := inbox.NewMockInboxRepo(ctrl)
	mockClient := apiclient.NewMockClient(ctrl)

	w := newTestWorker(mockRepo, mockClient)

	msg := inbox.InboxMessage{
		ID:          "msg-1",
		WebhookType: "order_update",
		Payload:     orderPayload(t),
	}

	mockClient.EXPECT().
		SendOrderUpdate(gomock.Any(), gomock.Any()).
		Return(nil)

	err := w.processMessage(context.Background(), msg)
	assert.NoError(t, err)
}

func TestProcessMessage_DisputeUpdate_Success(t *testing.T) {
	ctrl := gomock.NewController(t)
	mockRepo := inbox.NewMockInboxRepo(ctrl)
	mockClient := apiclient.NewMockClient(ctrl)

	w := newTestWorker(mockRepo, mockClient)

	msg := inbox.InboxMessage{
		ID:          "msg-2",
		WebhookType: "dispute_update",
		Payload:     disputePayload(t),
	}

	mockClient.EXPECT().
		SendDisputeUpdate(gomock.Any(), gomock.Any()).
		Return(nil)

	err := w.processMessage(context.Background(), msg)
	assert.NoError(t, err)
}

func TestProcessMessage_Conflict_TreatedAsSuccess(t *testing.T) {
	ctrl := gomock.NewController(t)
	mockRepo := inbox.NewMockInboxRepo(ctrl)
	mockClient := apiclient.NewMockClient(ctrl)

	w := newTestWorker(mockRepo, mockClient)

	msg := inbox.InboxMessage{
		ID:          "msg-3",
		WebhookType: "order_update",
		Payload:     orderPayload(t),
	}

	mockClient.EXPECT().
		SendOrderUpdate(gomock.Any(), gomock.Any()).
		Return(fmt.Errorf("%w: already exists", apiclient.ErrConflict))

	err := w.processMessage(context.Background(), msg)
	assert.NoError(t, err, "ErrConflict should be treated as idempotent success")
}

func TestProcessMessage_ServiceUnavailable_ReturnsError(t *testing.T) {
	ctrl := gomock.NewController(t)
	mockRepo := inbox.NewMockInboxRepo(ctrl)
	mockClient := apiclient.NewMockClient(ctrl)

	w := newTestWorker(mockRepo, mockClient)

	msg := inbox.InboxMessage{
		ID:          "msg-4",
		WebhookType: "order_update",
		Payload:     orderPayload(t),
	}

	mockClient.EXPECT().
		SendOrderUpdate(gomock.Any(), gomock.Any()).
		Return(fmt.Errorf("%w: timeout", apiclient.ErrServiceUnavailable))

	err := w.processMessage(context.Background(), msg)
	assert.Error(t, err)
	assert.False(t, isPermanentError(err), "ErrServiceUnavailable is transient")
}

func TestProcessMessage_BadRequest_PermanentError(t *testing.T) {
	ctrl := gomock.NewController(t)
	mockRepo := inbox.NewMockInboxRepo(ctrl)
	mockClient := apiclient.NewMockClient(ctrl)

	w := newTestWorker(mockRepo, mockClient)

	msg := inbox.InboxMessage{
		ID:          "msg-5",
		WebhookType: "order_update",
		Payload:     orderPayload(t),
	}

	mockClient.EXPECT().
		SendOrderUpdate(gomock.Any(), gomock.Any()).
		Return(fmt.Errorf("%w: invalid json", apiclient.ErrBadRequest))

	err := w.processMessage(context.Background(), msg)
	assert.Error(t, err)
	assert.True(t, isPermanentError(err), "ErrBadRequest is permanent")
}

func TestProcessMessage_UnknownWebhookType_ReturnsError(t *testing.T) {
	ctrl := gomock.NewController(t)
	mockRepo := inbox.NewMockInboxRepo(ctrl)
	mockClient := apiclient.NewMockClient(ctrl)

	w := newTestWorker(mockRepo, mockClient)

	msg := inbox.InboxMessage{
		ID:          "msg-6",
		WebhookType: "unknown_type",
		Payload:     json.RawMessage(`{}`),
	}

	err := w.processMessage(context.Background(), msg)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "unknown webhook type")
}

func TestPoll_EmptyBatch_NoClientCalls(t *testing.T) {
	ctrl := gomock.NewController(t)
	mockRepo := inbox.NewMockInboxRepo(ctrl)
	mockClient := apiclient.NewMockClient(ctrl)

	w := newTestWorker(mockRepo, mockClient)

	mockRepo.EXPECT().
		FetchPending(gomock.Any(), w.cfg.BatchSize).
		Return(nil, nil)

	// No client calls expected
	w.poll(context.Background())
}

func TestPoll_SuccessfulMessage_MarkedProcessed(t *testing.T) {
	ctrl := gomock.NewController(t)
	mockRepo := inbox.NewMockInboxRepo(ctrl)
	mockClient := apiclient.NewMockClient(ctrl)

	w := newTestWorker(mockRepo, mockClient)

	msg := inbox.InboxMessage{
		ID:          "msg-7",
		WebhookType: "order_update",
		Payload:     orderPayload(t),
	}

	mockRepo.EXPECT().
		FetchPending(gomock.Any(), w.cfg.BatchSize).
		Return([]inbox.InboxMessage{msg}, nil)

	mockClient.EXPECT().
		SendOrderUpdate(gomock.Any(), gomock.Any()).
		Return(nil)

	mockRepo.EXPECT().
		MarkProcessed(gomock.Any(), "msg-7").
		Return(nil)

	w.poll(context.Background())
}

func TestPoll_FailedMessage_MarkedFailed(t *testing.T) {
	ctrl := gomock.NewController(t)
	mockRepo := inbox.NewMockInboxRepo(ctrl)
	mockClient := apiclient.NewMockClient(ctrl)

	w := newTestWorker(mockRepo, mockClient)

	msg := inbox.InboxMessage{
		ID:          "msg-8",
		WebhookType: "order_update",
		Payload:     orderPayload(t),
	}

	mockRepo.EXPECT().
		FetchPending(gomock.Any(), w.cfg.BatchSize).
		Return([]inbox.InboxMessage{msg}, nil)

	mockClient.EXPECT().
		SendOrderUpdate(gomock.Any(), gomock.Any()).
		Return(fmt.Errorf("%w: timeout", apiclient.ErrServiceUnavailable))

	mockRepo.EXPECT().
		MarkFailed(gomock.Any(), "msg-8", gomock.Any(), w.cfg.MaxRetries).
		Return(nil)

	w.poll(context.Background())
}

func TestPoll_PermanentError_MarkedFailedWithZeroRetries(t *testing.T) {
	ctrl := gomock.NewController(t)
	mockRepo := inbox.NewMockInboxRepo(ctrl)
	mockClient := apiclient.NewMockClient(ctrl)

	w := newTestWorker(mockRepo, mockClient)

	msg := inbox.InboxMessage{
		ID:          "msg-9",
		WebhookType: "order_update",
		Payload:     orderPayload(t),
	}

	mockRepo.EXPECT().
		FetchPending(gomock.Any(), w.cfg.BatchSize).
		Return([]inbox.InboxMessage{msg}, nil)

	mockClient.EXPECT().
		SendOrderUpdate(gomock.Any(), gomock.Any()).
		Return(fmt.Errorf("%w: bad payload", apiclient.ErrBadRequest))

	// maxRetries=0 forces immediate 'failed' status
	mockRepo.EXPECT().
		MarkFailed(gomock.Any(), "msg-9", gomock.Any(), 0).
		Return(nil)

	w.poll(context.Background())
}

func TestStart_StopsOnContextCancel(t *testing.T) {
	ctrl := gomock.NewController(t)
	mockRepo := inbox.NewMockInboxRepo(ctrl)
	mockClient := apiclient.NewMockClient(ctrl)

	w := newTestWorker(mockRepo, mockClient)

	// Allow any number of FetchPending calls (worker polls until cancelled)
	mockRepo.EXPECT().
		FetchPending(gomock.Any(), gomock.Any()).
		Return(nil, nil).
		AnyTimes()

	ctx, cancel := context.WithCancel(context.Background())

	done := make(chan error, 1)
	go func() {
		done <- w.Start(ctx)
	}()

	// Let it run at least one tick
	time.Sleep(100 * time.Millisecond)
	cancel()

	select {
	case err := <-done:
		assert.ErrorIs(t, err, context.Canceled)
	case <-time.After(2 * time.Second):
		t.Fatal("worker did not stop within timeout")
	}
}
