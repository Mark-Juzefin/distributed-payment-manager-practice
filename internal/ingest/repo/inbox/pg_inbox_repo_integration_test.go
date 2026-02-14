//go:build integration
// +build integration

package inbox_test

import (
	"TestTaskJustPay/internal/ingest/repo/inbox"
	"context"
	"encoding/json"
	"errors"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

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
