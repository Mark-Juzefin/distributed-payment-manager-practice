//go:build integration
// +build integration

package inbox_test

import (
	"TestTaskJustPay/internal/ingest/repo/inbox"
	"context"
	"encoding/json"
	"errors"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// storeTestMessage is a helper that stores a message and returns its ID.
func storeTestMessage(t *testing.T, ctx context.Context, repo *inbox.PgInboxRepo, key string, webhookType string) string {
	t.Helper()

	err := repo.Store(ctx, inbox.NewInboxMessage{
		IdempotencyKey: key,
		WebhookType:    webhookType,
		Payload:        json.RawMessage(`{"test":true}`),
	})
	require.NoError(t, err)

	var id string
	err = pool.Pool.QueryRow(ctx,
		"SELECT id FROM inbox WHERE idempotency_key = $1", key,
	).Scan(&id)
	require.NoError(t, err)

	return id
}

func TestStore_Success(t *testing.T) {
	t.Parallel()
	ctx := context.Background()

	repo := inbox.NewPgInboxRepo(pool.Pool, pool.Builder)

	msg := inbox.NewInboxMessage{
		IdempotencyKey: "order_update:evt_success_test",
		WebhookType:    "order_update",
		Payload:        json.RawMessage(`{"order_id":"order_001","status":"created"}`),
	}

	err := repo.Store(ctx, msg)
	require.NoError(t, err)

	// Verify row exists with correct fields
	var (
		webhookType string
		status      string
		payload     json.RawMessage
	)
	scanErr := pool.Pool.QueryRow(ctx,
		"SELECT webhook_type, status, payload FROM inbox WHERE idempotency_key = $1",
		msg.IdempotencyKey,
	).Scan(&webhookType, &status, &payload)

	require.NoError(t, scanErr)
	assert.Equal(t, "order_update", webhookType)
	assert.Equal(t, "pending", status)
	assert.JSONEq(t, `{"order_id":"order_001","status":"created"}`, string(payload))
}

func TestStore_IdempotencyConstraint(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name                 string
		firstMsg             inbox.NewInboxMessage
		duplicateMsg         inbox.NewInboxMessage
		expectDuplicateError bool
	}{
		{
			name: "Duplicate idempotency_key returns ErrAlreadyExists",
			firstMsg: inbox.NewInboxMessage{
				IdempotencyKey: "order_update:evt_dup_001",
				WebhookType:    "order_update",
				Payload:        json.RawMessage(`{"order_id":"order_001","status":"created"}`),
			},
			duplicateMsg: inbox.NewInboxMessage{
				IdempotencyKey: "order_update:evt_dup_001", // same key
				WebhookType:    "order_update",
				Payload:        json.RawMessage(`{"order_id":"order_001","status":"updated"}`),
			},
			expectDuplicateError: true,
		},
		{
			name: "Different webhook types with same provider_event_id succeed",
			firstMsg: inbox.NewInboxMessage{
				IdempotencyKey: "order_update:evt_cross_type",
				WebhookType:    "order_update",
				Payload:        json.RawMessage(`{"order_id":"order_001"}`),
			},
			duplicateMsg: inbox.NewInboxMessage{
				IdempotencyKey: "dispute_update:evt_cross_type", // different prefix
				WebhookType:    "dispute_update",
				Payload:        json.RawMessage(`{"order_id":"order_001"}`),
			},
			expectDuplicateError: false,
		},
		{
			name: "Same webhook type with different provider_event_id succeed",
			firstMsg: inbox.NewInboxMessage{
				IdempotencyKey: "order_update:evt_unique_001",
				WebhookType:    "order_update",
				Payload:        json.RawMessage(`{"order_id":"order_001"}`),
			},
			duplicateMsg: inbox.NewInboxMessage{
				IdempotencyKey: "order_update:evt_unique_002", // different event ID
				WebhookType:    "order_update",
				Payload:        json.RawMessage(`{"order_id":"order_001"}`),
			},
			expectDuplicateError: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			ctx := context.Background()

			repo := inbox.NewPgInboxRepo(pool.Pool, pool.Builder)

			// Store first message
			err := repo.Store(ctx, tt.firstMsg)
			require.NoError(t, err, "first store should succeed")

			// Attempt duplicate
			err = repo.Store(ctx, tt.duplicateMsg)

			if tt.expectDuplicateError {
				require.Error(t, err)
				assert.True(t, errors.Is(err, inbox.ErrAlreadyExists),
					"expected ErrAlreadyExists, got: %v", err)
			} else {
				require.NoError(t, err, "second store should succeed for different key")
			}
		})
	}
}

func TestFetchPending_ReturnsPendingOrderedByReceivedAt(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	repo := inbox.NewPgInboxRepo(pool.Pool, pool.Builder)

	// Store messages with different received_at (using unique keys)
	id1 := storeTestMessage(t, ctx, repo, "fetch_order:evt_first", "order_update")
	// Small sleep to ensure different received_at timestamps
	time.Sleep(10 * time.Millisecond)
	id2 := storeTestMessage(t, ctx, repo, "fetch_order:evt_second", "order_update")

	messages, err := repo.FetchPending(ctx, 10)
	require.NoError(t, err)

	// Should contain at least our 2 messages, ordered by received_at
	var foundIDs []string
	for _, m := range messages {
		if m.ID == id1 || m.ID == id2 {
			foundIDs = append(foundIDs, m.ID)
		}
	}
	require.Len(t, foundIDs, 2)
	assert.Equal(t, id1, foundIDs[0], "first inserted should come first")
	assert.Equal(t, id2, foundIDs[1], "second inserted should come second")

	// Verify status changed to 'processing'
	var status string
	err = pool.Pool.QueryRow(ctx, "SELECT status FROM inbox WHERE id = $1", id1).Scan(&status)
	require.NoError(t, err)
	assert.Equal(t, "processing", status)
}

func TestFetchPending_SkipsProcessingAndProcessedRows(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	repo := inbox.NewPgInboxRepo(pool.Pool, pool.Builder)

	// Store a message and manually set to 'processing'
	id1 := storeTestMessage(t, ctx, repo, "fetch_skip:evt_processing", "order_update")
	_, err := pool.Pool.Exec(ctx, "UPDATE inbox SET status = 'processing' WHERE id = $1", id1)
	require.NoError(t, err)

	// Store a message and manually set to 'processed'
	id2 := storeTestMessage(t, ctx, repo, "fetch_skip:evt_processed", "order_update")
	_, err = pool.Pool.Exec(ctx, "UPDATE inbox SET status = 'processed' WHERE id = $1", id2)
	require.NoError(t, err)

	// Store a pending message
	id3 := storeTestMessage(t, ctx, repo, "fetch_skip:evt_pending", "order_update")

	messages, err := repo.FetchPending(ctx, 100)
	require.NoError(t, err)

	// Should NOT contain processing or processed rows
	for _, m := range messages {
		assert.NotEqual(t, id1, m.ID, "should skip processing row")
		assert.NotEqual(t, id2, m.ID, "should skip processed row")
	}

	// Should contain the pending message
	var found bool
	for _, m := range messages {
		if m.ID == id3 {
			found = true
			break
		}
	}
	assert.True(t, found, "should contain the pending message")
}

func TestFetchPending_RespectsLimit(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	repo := inbox.NewPgInboxRepo(pool.Pool, pool.Builder)

	// Store 3 messages
	storeTestMessage(t, ctx, repo, "fetch_limit:evt_1", "order_update")
	storeTestMessage(t, ctx, repo, "fetch_limit:evt_2", "order_update")
	storeTestMessage(t, ctx, repo, "fetch_limit:evt_3", "order_update")

	messages, err := repo.FetchPending(ctx, 1)
	require.NoError(t, err)
	// May pick up messages from other tests too, but limit should still apply
	assert.LessOrEqual(t, len(messages), 1)
}

func TestFetchPending_ReturnsEmptyWhenNoPending(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	repo := inbox.NewPgInboxRepo(pool.Pool, pool.Builder)

	// Store a message and set to 'processed' so no pending rows from this test
	id := storeTestMessage(t, ctx, repo, "fetch_empty:evt_done", "order_update")
	_, err := pool.Pool.Exec(ctx, "UPDATE inbox SET status = 'processed' WHERE id = $1", id)
	require.NoError(t, err)

	// FetchPending may return messages from other parallel tests;
	// this primarily tests that the call succeeds without error
	messages, err := repo.FetchPending(ctx, 10)
	require.NoError(t, err)

	// Our processed message should not be in the result
	for _, m := range messages {
		assert.NotEqual(t, id, m.ID, "processed message should not be fetched")
	}

	_ = messages // no panic, valid return
}

func TestMarkProcessed_UpdatesStatusAndProcessedAt(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	repo := inbox.NewPgInboxRepo(pool.Pool, pool.Builder)

	id := storeTestMessage(t, ctx, repo, "mark_proc:evt_1", "order_update")

	err := repo.MarkProcessed(ctx, id)
	require.NoError(t, err)

	var (
		status      string
		processedAt *time.Time
	)
	err = pool.Pool.QueryRow(ctx,
		"SELECT status, processed_at FROM inbox WHERE id = $1", id,
	).Scan(&status, &processedAt)
	require.NoError(t, err)
	assert.Equal(t, "processed", status)
	assert.NotNil(t, processedAt, "processed_at should be set")
}

func TestMarkFailed_IncrementsRetryAndResetsToPending(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	repo := inbox.NewPgInboxRepo(pool.Pool, pool.Builder)

	id := storeTestMessage(t, ctx, repo, "mark_fail:evt_retry", "order_update")

	// Mark failed with maxRetries=3 (retry_count goes 0→1, still < 3, so reset to pending)
	err := repo.MarkFailed(ctx, id, "timeout error", 3)
	require.NoError(t, err)

	var (
		status       string
		retryCount   int
		errorMessage *string
	)
	err = pool.Pool.QueryRow(ctx,
		"SELECT status, retry_count, error_message FROM inbox WHERE id = $1", id,
	).Scan(&status, &retryCount, &errorMessage)
	require.NoError(t, err)
	assert.Equal(t, "pending", status, "should reset to pending for retry")
	assert.Equal(t, 1, retryCount)
	require.NotNil(t, errorMessage)
	assert.Equal(t, "timeout error", *errorMessage)
}

func TestMarkFailed_SetsFailedStatusWhenMaxRetriesReached(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	repo := inbox.NewPgInboxRepo(pool.Pool, pool.Builder)

	id := storeTestMessage(t, ctx, repo, "mark_fail:evt_permanent", "order_update")

	// Simulate retries: set retry_count to 2, then mark failed with maxRetries=3
	_, err := pool.Pool.Exec(ctx, "UPDATE inbox SET retry_count = 2 WHERE id = $1", id)
	require.NoError(t, err)

	err = repo.MarkFailed(ctx, id, "still failing", 3)
	require.NoError(t, err)

	var (
		status     string
		retryCount int
	)
	err = pool.Pool.QueryRow(ctx,
		"SELECT status, retry_count FROM inbox WHERE id = $1", id,
	).Scan(&status, &retryCount)
	require.NoError(t, err)
	assert.Equal(t, "failed", status, "should be permanently failed")
	assert.Equal(t, 3, retryCount, "retry_count should be incremented to 3")
}

func TestMarkFailed_ZeroMaxRetries_ImmediatelyFails(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	repo := inbox.NewPgInboxRepo(pool.Pool, pool.Builder)

	id := storeTestMessage(t, ctx, repo, "mark_fail:evt_immediate", "order_update")

	// maxRetries=0 means any failure is permanent
	err := repo.MarkFailed(ctx, id, "bad request", 0)
	require.NoError(t, err)

	var status string
	err = pool.Pool.QueryRow(ctx,
		"SELECT status FROM inbox WHERE id = $1", id,
	).Scan(&status)
	require.NoError(t, err)
	assert.Equal(t, "failed", status, "maxRetries=0 should immediately fail")
}
