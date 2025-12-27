//go:build integration
// +build integration

package dispute_eventsink_test

import (
	"TestTaskJustPay/internal/controller/apperror"
	"TestTaskJustPay/internal/domain/dispute"
	"TestTaskJustPay/internal/repo/dispute_eventsink"
	"TestTaskJustPay/pkg/postgres"
	"context"
	_ "embed"
	"errors"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestCreateDisputeEventIntegration(t *testing.T) {
	t.Parallel()
	ctx := context.Background()

	tests := []struct {
		name        string
		seed        func(t *testing.T, tx postgres.Executor)
		event       dispute.NewDisputeEvent
		expectError bool
		errorMsg    string
	}{
		{
			name: "Create dispute event successfully",
			seed: func(t *testing.T, tx postgres.Executor) {
				applyBaseFixture(t, tx)
			},
			event: dispute.NewDisputeEvent{
				DisputeID:       "dispute_001",
				Kind:            dispute.DisputeEventWebhookUpdated,
				ProviderEventID: "new_provider_event_123",
				Data:            []byte(`{"status": "new_status", "amount": 150.00}`),
				CreatedAt:       time.Date(2024, 1, 30, 10, 0, 0, 0, time.UTC),
			},
			expectError: false,
		},
		{
			name: "Create event for non-existent dispute",
			seed: func(t *testing.T, tx postgres.Executor) {
				applyBaseFixture(t, tx)
			},
			event: dispute.NewDisputeEvent{
				DisputeID:       "non_existent_dispute",
				Kind:            dispute.DisputeEventWebhookOpened,
				ProviderEventID: "provider_event_999",
				Data:            []byte(`{"amount": 100.00}`),
				CreatedAt:       time.Date(2024, 1, 30, 11, 0, 0, 0, time.UTC),
			},
			expectError: true,
			errorMsg:    "foreign key",
		},
		{
			name: "Create multiple events for same dispute",
			seed: func(t *testing.T, tx postgres.Executor) {
				applyBaseFixture(t, tx)
				// First, create one event
				repo := dispute_eventsink.NewPgEventRepo(tx, pool.Builder)
				firstEvent := dispute.NewDisputeEvent{
					DisputeID:       "dispute_001",
					Kind:            dispute.DisputeEventEvidenceAdded,
					ProviderEventID: "first_additional_event",
					Data:            []byte(`{"document_type": "first_doc"}`),
					CreatedAt:       time.Date(2024, 1, 30, 13, 0, 0, 0, time.UTC),
				}
				_, err := repo.CreateDisputeEvent(ctx, firstEvent)
				require.NoError(t, err)
			},
			event: dispute.NewDisputeEvent{
				DisputeID:       "dispute_001",
				Kind:            dispute.DisputeEventEvidenceAdded,
				ProviderEventID: "second_additional_event",
				Data:            []byte(`{"document_type": "second_doc"}`),
				CreatedAt:       time.Date(2024, 1, 30, 14, 0, 0, 0, time.UTC),
			},
			expectError: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := pool.SandboxTransaction(ctx, func(tx postgres.Executor) error {
				tt.seed(t, tx)

				repo := dispute_eventsink.NewPgEventRepo(tx, pool.Builder)
				createdEvent, err := repo.CreateDisputeEvent(ctx, tt.event)

				if tt.expectError {
					require.Error(t, err)
					require.Nil(t, createdEvent)
					if tt.errorMsg != "" {
						assert.Contains(t, err.Error(), tt.errorMsg)
					}
				} else {
					require.NoError(t, err)
					require.NotNil(t, createdEvent)

					// Verify the returned event has correct data
					assert.Equal(t, tt.event, createdEvent.NewDisputeEvent)

					// Verify the event can be retrieved by ID
					retrievedEvent, err := repo.GetDisputeEventByID(ctx, createdEvent.EventID)
					require.NoError(t, err)
					require.NotNil(t, retrievedEvent)

					assertDisputeEventEqual(t, createdEvent, retrievedEvent)
				}
				return nil
			})
			require.NoError(t, err)
		})
	}
}

func TestGetDisputeEventByIDIntegration(t *testing.T) {
	t.Parallel()
	ctx := context.Background()

	tests := []struct {
		name        string
		seed        func(t *testing.T, tx postgres.Executor)
		eventID     string
		expectError bool
		errorMsg    string
		validate    func(t *testing.T, event *dispute.DisputeEvent)
	}{
		{
			name: "Get existing event by ID",
			seed: func(t *testing.T, tx postgres.Executor) {
				applyBaseFixture(t, tx)
			},
			eventID:     "event_001", // From base fixture
			expectError: false,
			validate: func(t *testing.T, event *dispute.DisputeEvent) {
				assert.Equal(t, "event_001", event.EventID)
				assert.Equal(t, "dispute_001", event.DisputeID)
				assert.Equal(t, dispute.DisputeEventWebhookOpened, event.Kind)
				assert.Equal(t, "chb_opened_001", event.ProviderEventID)
				assert.NotEmpty(t, event.Data)
				assert.NotZero(t, event.CreatedAt)
			},
		},
		{
			name: "Get non-existent event by ID",
			seed: func(t *testing.T, tx postgres.Executor) {
				applyEdgeCasesFixture(t, tx)
			},
			eventID:     "non_existent_event_id",
			expectError: true,
			errorMsg:    "dispute event not found",
		},
		{
			name: "Get event from edge case fixture",
			seed: func(t *testing.T, tx postgres.Executor) {
				applyEdgeCasesFixture(t, tx)
			},
			eventID:     "edge_event_001",
			expectError: false,
			validate: func(t *testing.T, event *dispute.DisputeEvent) {
				assert.Equal(t, "edge_event_001", event.EventID)
				assert.Equal(t, "edge_dispute_001", event.DisputeID)
				assert.Equal(t, dispute.DisputeEventWebhookOpened, event.Kind)
				assert.Equal(t, "edge_provider_001", event.ProviderEventID)
				assert.Contains(t, string(event.Data), "single_event")
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := pool.SandboxTransaction(ctx, func(tx postgres.Executor) error {
				tt.seed(t, tx)

				repo := dispute_eventsink.NewPgEventRepo(tx, pool.Builder)
				event, err := repo.GetDisputeEventByID(ctx, tt.eventID)

				if tt.expectError {
					require.Error(t, err)
					require.Nil(t, event)
					if tt.errorMsg != "" {
						assert.Contains(t, err.Error(), tt.errorMsg)
					}
				} else {
					require.NoError(t, err)
					require.NotNil(t, event)
					if tt.validate != nil {
						tt.validate(t, event)
					}
				}
				return nil
			})
			require.NoError(t, err)
		})
	}
}

// TODO: add testcases for pagination cursor
func TestGetDisputeEventsIntegration(t *testing.T) {
	t.Parallel()
	ctx := context.Background()

	// Test cases for basic functionality
	basicTests := []struct {
		name        string
		seed        func(t *testing.T, tx postgres.Executor)
		query       dispute.DisputeEventQuery
		validate    func(t *testing.T, result dispute.DisputeEventPage)
		expectError bool
		errorMsg    string
	}{
		{
			name: "Get all events without filters",
			seed: func(t *testing.T, tx postgres.Executor) {
				applyEdgeCasesFixture(t, tx)
			},
			query: dispute.DisputeEventQuery{
				Limit: 100,
			},
			validate: func(t *testing.T, result dispute.DisputeEventPage) {
				assert.Equal(t, 1, len(result.Items)) // Only 1 event in edge_cases fixture
				assert.Equal(t, "edge_event_001", result.Items[0].EventID)
				assert.Equal(t, "edge_dispute_001", result.Items[0].DisputeID)
				assert.Equal(t, dispute.DisputeEventWebhookOpened, result.Items[0].Kind)
				assert.False(t, result.HasMore)
				assert.Empty(t, result.NextCursor)
			},
		},
		{
			name: "Get events for specific dispute ID",
			seed: func(t *testing.T, tx postgres.Executor) {
				applyBaseFixture(t, tx)
			},
			query: dispute.DisputeEventQuery{
				DisputeIDs: []string{"dispute_001"},
				Limit:      100,
			},
			validate: func(t *testing.T, result dispute.DisputeEventPage) {
				assert.Equal(t, 5, len(result.Items)) // dispute_001 has 5 events in minimal fixture
				for _, event := range result.Items {
					assert.Equal(t, "dispute_001", event.DisputeID)
				}
				assert.False(t, result.HasMore)
			},
		},
		{
			name: "Get events for multiple dispute IDs",
			seed: func(t *testing.T, tx postgres.Executor) {
				applyBaseFixture(t, tx)
			},
			query: dispute.DisputeEventQuery{
				DisputeIDs: []string{"dispute_001", "dispute_002"},
				Limit:      100,
			},
			validate: func(t *testing.T, result dispute.DisputeEventPage) {
				assert.Equal(t, 10, len(result.Items)) // dispute_001 (5) + dispute_002 (5) = 10 events
				for _, event := range result.Items {
					assert.Contains(t, []string{"dispute_001", "dispute_002"}, event.DisputeID)
				}
			},
		},
		{
			name: "Get events for non-existent dispute ID",
			seed: func(t *testing.T, tx postgres.Executor) {
				applyBaseFixture(t, tx)
			},
			query: dispute.DisputeEventQuery{
				DisputeIDs: []string{"non_existent_dispute"},
				Limit:      100,
			},
			validate: func(t *testing.T, result dispute.DisputeEventPage) {
				assert.Equal(t, 0, len(result.Items))
				assert.False(t, result.HasMore)
				assert.Empty(t, result.NextCursor)
			},
		},
		{
			name: "Filter by single event kind",
			seed: func(t *testing.T, tx postgres.Executor) {
				applyFilteringFixture(t, tx)
			},
			query: dispute.DisputeEventQuery{
				Kinds: []dispute.DisputeEventKind{dispute.DisputeEventEvidenceAdded},
				Limit: 100,
			},
			validate: func(t *testing.T, result dispute.DisputeEventPage) {
				assert.Equal(t, 6, len(result.Items)) // 6 evidence_added events in filtering fixture
				for _, event := range result.Items {
					assert.Equal(t, dispute.DisputeEventEvidenceAdded, event.Kind)
				}
			},
		},
		{
			name: "Filter by multiple event kinds",
			seed: func(t *testing.T, tx postgres.Executor) {
				applyFilteringFixture(t, tx)
			},
			query: dispute.DisputeEventQuery{
				Kinds: []dispute.DisputeEventKind{dispute.DisputeEventWebhookOpened, dispute.DisputeEventProviderDecision},
				Limit: 100,
			},
			validate: func(t *testing.T, result dispute.DisputeEventPage) {
				assert.Equal(t, 5, len(result.Items)) // 3 webhook_opened + 2 provider_decision
				for _, event := range result.Items {
					assert.Contains(t, []dispute.DisputeEventKind{dispute.DisputeEventWebhookOpened, dispute.DisputeEventProviderDecision}, event.Kind)
				}
			},
		},
		{
			name: "Default limit applied when not specified",
			seed: func(t *testing.T, tx postgres.Executor) {
				applyPaginationFixture(t, tx)
			},
			query: dispute.DisputeEventQuery{
				// No limit specified, should default to 10
			},
			validate: func(t *testing.T, result dispute.DisputeEventPage) {
				assert.Equal(t, 10, len(result.Items)) // Default limit is 10
				assert.True(t, result.HasMore)         // Should have more since pagination fixture has 25 events
				assert.NotEmpty(t, result.NextCursor)
			},
		},
	}

	for _, tt := range basicTests {
		t.Run(tt.name, func(t *testing.T) {
			err := pool.SandboxTransaction(ctx, func(tx postgres.Executor) error {
				tt.seed(t, tx)

				repo := dispute_eventsink.NewPgEventRepo(tx, pool.Builder)
				result, err := repo.GetDisputeEvents(ctx, tt.query)

				if tt.expectError {
					require.Error(t, err)
					if tt.errorMsg != "" {
						assert.Contains(t, err.Error(), tt.errorMsg)
					}
				} else {
					require.NoError(t, err)
					tt.validate(t, result)
				}
				return nil
			})
			require.NoError(t, err)
		})
	}
}

func TestCreateDisputeEvent_IdempotencyConstraint(t *testing.T) {
	t.Parallel()
	ctx := context.Background()

	tests := []struct {
		name                 string
		seed                 func(t *testing.T, tx postgres.Executor)
		firstEvent           dispute.NewDisputeEvent
		duplicateEvent       dispute.NewDisputeEvent
		expectDuplicateError bool
	}{
		{
			name: "Duplicate provider_event_id for same dispute returns ErrEventAlreadyStored",
			seed: func(t *testing.T, tx postgres.Executor) {
				applyBaseFixture(t, tx)
			},
			firstEvent: dispute.NewDisputeEvent{
				DisputeID:       "dispute_001",
				Kind:            dispute.DisputeEventWebhookOpened,
				ProviderEventID: "provider_evt_123",
				Data:            []byte(`{"status": "opened"}`),
				CreatedAt:       time.Date(2024, 1, 15, 10, 0, 0, 0, time.UTC),
			},
			duplicateEvent: dispute.NewDisputeEvent{
				DisputeID:       "dispute_001",
				Kind:            dispute.DisputeEventWebhookUpdated,
				ProviderEventID: "provider_evt_123",                            // Same provider_event_id
				Data:            []byte(`{"status": "updated"}`),               // Different data
				CreatedAt:       time.Date(2024, 1, 15, 10, 0, 0, 0, time.UTC), // SAME created_at for true duplicate
			},
			expectDuplicateError: true,
		},
		{
			name: "Same provider_event_id for different disputes succeeds",
			seed: func(t *testing.T, tx postgres.Executor) {
				applyBaseFixture(t, tx)
			},
			firstEvent: dispute.NewDisputeEvent{
				DisputeID:       "dispute_001",
				Kind:            dispute.DisputeEventWebhookOpened,
				ProviderEventID: "provider_evt_shared",
				Data:            []byte(`{"status": "opened"}`),
				CreatedAt:       time.Date(2024, 1, 15, 10, 0, 0, 0, time.UTC),
			},
			duplicateEvent: dispute.NewDisputeEvent{
				DisputeID:       "dispute_002", // Different dispute
				Kind:            dispute.DisputeEventWebhookOpened,
				ProviderEventID: "provider_evt_shared", // Same provider_event_id (different disputes can have same)
				Data:            []byte(`{"status": "opened"}`),
				CreatedAt:       time.Date(2024, 1, 15, 11, 0, 0, 0, time.UTC),
			},
			expectDuplicateError: false,
		},
		{
			name: "Different provider_event_id for same dispute succeeds",
			seed: func(t *testing.T, tx postgres.Executor) {
				applyBaseFixture(t, tx)
			},
			firstEvent: dispute.NewDisputeEvent{
				DisputeID:       "dispute_001",
				Kind:            dispute.DisputeEventWebhookOpened,
				ProviderEventID: "provider_evt_first",
				Data:            []byte(`{"status": "opened"}`),
				CreatedAt:       time.Date(2024, 1, 15, 10, 0, 0, 0, time.UTC),
			},
			duplicateEvent: dispute.NewDisputeEvent{
				DisputeID:       "dispute_001",
				Kind:            dispute.DisputeEventWebhookUpdated,
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

				repo := dispute_eventsink.NewPgEventRepo(tx, pool.Builder)

				// Create first event
				firstCreated, err := repo.CreateDisputeEvent(ctx, tt.firstEvent)
				require.NoError(t, err)
				require.NotNil(t, firstCreated)
				assert.Equal(t, tt.firstEvent.DisputeID, firstCreated.DisputeID)
				assert.Equal(t, tt.firstEvent.ProviderEventID, firstCreated.ProviderEventID)

				// Attempt to create duplicate event
				duplicateCreated, err := repo.CreateDisputeEvent(ctx, tt.duplicateEvent)

				if tt.expectDuplicateError {
					// Should return ErrEventAlreadyStored for duplicate (dispute_id, provider_event_id)
					require.Error(t, err)
					assert.True(t, errors.Is(err, apperror.ErrEventAlreadyStored),
						"Expected ErrEventAlreadyStored, got: %v", err)
					assert.Nil(t, duplicateCreated)
				} else {
					// Should succeed for different combinations
					require.NoError(t, err)
					require.NotNil(t, duplicateCreated)
					assert.Equal(t, tt.duplicateEvent.DisputeID, duplicateCreated.DisputeID)
					assert.Equal(t, tt.duplicateEvent.ProviderEventID, duplicateCreated.ProviderEventID)
				}

				return nil
			})

			require.NoError(t, err)
		})
	}
}

func assertDisputeEventEqual(t *testing.T, exp, act *dispute.DisputeEvent) {
	t.Helper()
	assert.Equal(t, exp.EventID, act.EventID)
	assert.Equal(t, exp.DisputeID, act.DisputeID)
	assert.Equal(t, exp.Kind, act.Kind)
	assert.Equal(t, exp.ProviderEventID, act.ProviderEventID)
	assert.True(t, exp.CreatedAt.Equal(act.CreatedAt))
	assert.JSONEq(t, string(exp.Data), string(act.Data))
}
