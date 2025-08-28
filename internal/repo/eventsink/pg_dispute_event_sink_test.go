package eventsink

import (
	"TestTaskJustPay/internal/domain/dispute"
	"testing"
	"time"

	"github.com/Masterminds/squirrel"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestPgEventRepo_buildDisputeEventPageQuery(t *testing.T) {
	// Create a test instance with a mock builder
	repo := &PgDisputeEventRepo{
		builder: squirrel.StatementBuilder.PlaceholderFormat(squirrel.Dollar),
	}

	// Test cursor for pagination
	testTime := time.Date(2024, 1, 15, 10, 30, 0, 0, time.UTC)
	testCursor := encodeEventCursor(eventCursor{
		EventID:   "event_123",
		CreatedAt: testTime,
	})

	tests := []struct {
		name          string
		query         dispute.DisputeEventQuery
		expectedSQL   string
		expectedArgs  []interface{}
		shouldError   bool
		errorContains string
	}{
		{
			name: "basic query with dispute IDs only",
			query: dispute.DisputeEventQuery{
				DisputeIDs: []string{"dispute_1", "dispute_2"},
				Limit:      10,
			},
			expectedSQL: "SELECT id, dispute_id, kind, provider_event_id, data, created_at FROM dispute_events WHERE dispute_id IN ($1,$2) ORDER BY created_at DESC, id DESC LIMIT 11",
			expectedArgs: []interface{}{
				"dispute_1", "dispute_2",
			},
		},
		{
			name: "query with kinds filter",
			query: dispute.DisputeEventQuery{
				DisputeIDs: []string{"dispute_1"},
				Kinds:      []dispute.DisputeEventKind{dispute.DisputeEventWebhookOpened, dispute.DisputeEventEvidenceAdded},
				Limit:      5,
			},
			expectedSQL: "SELECT id, dispute_id, kind, provider_event_id, data, created_at FROM dispute_events WHERE dispute_id IN ($1) AND kind IN ($2,$3) ORDER BY created_at DESC, id DESC LIMIT 6",
			expectedArgs: []interface{}{
				"dispute_1", dispute.DisputeEventWebhookOpened, dispute.DisputeEventEvidenceAdded,
			},
		},
		{
			name: "query with time range",
			query: dispute.DisputeEventQuery{
				DisputeIDs: []string{"dispute_1"},
				TimeFrom:   &testTime,
				TimeTo:     func() *time.Time { t := testTime.Add(24 * time.Hour); return &t }(),
				Limit:      10,
			},
			expectedSQL: "SELECT id, dispute_id, kind, provider_event_id, data, created_at FROM dispute_events WHERE dispute_id IN ($1) AND created_at >= $2 AND created_at < $3 ORDER BY created_at DESC, id DESC LIMIT 11",
			expectedArgs: []interface{}{
				"dispute_1", testTime.UTC(), testTime.Add(24 * time.Hour).UTC(),
			},
		},
		{
			name: "query with cursor pagination descending",
			query: dispute.DisputeEventQuery{
				DisputeIDs: []string{"dispute_1"},
				Cursor:     testCursor,
				SortAsc:    false,
				Limit:      10,
			},
			expectedSQL: "SELECT id, dispute_id, kind, provider_event_id, data, created_at FROM dispute_events WHERE dispute_id IN ($1) AND (created_at, id) < ($2, $3) ORDER BY created_at DESC, id DESC LIMIT 11",
			expectedArgs: []interface{}{
				"dispute_1", testTime.UTC(), "event_123",
			},
		},
		{
			name: "query with cursor pagination ascending",
			query: dispute.DisputeEventQuery{
				DisputeIDs: []string{"dispute_1"},
				Cursor:     testCursor,
				SortAsc:    true,
				Limit:      10,
			},
			expectedSQL: "SELECT id, dispute_id, kind, provider_event_id, data, created_at FROM dispute_events WHERE dispute_id IN ($1) AND (created_at, id) > ($2, $3) ORDER BY created_at ASC, id ASC LIMIT 11",
			expectedArgs: []interface{}{
				"dispute_1", testTime.UTC(), "event_123",
			},
		},
		{
			name: "comprehensive query with all filters",
			query: dispute.DisputeEventQuery{
				DisputeIDs: []string{"dispute_1", "dispute_2"},
				Kinds:      []dispute.DisputeEventKind{dispute.DisputeEventWebhookOpened},
				TimeFrom:   &testTime,
				TimeTo:     func() *time.Time { t := testTime.Add(24 * time.Hour); return &t }(),
				Cursor:     testCursor,
				SortAsc:    false,
				Limit:      20,
			},
			expectedSQL: "SELECT id, dispute_id, kind, provider_event_id, data, created_at FROM dispute_events WHERE dispute_id IN ($1,$2) AND kind IN ($3) AND created_at >= $4 AND created_at < $5 AND (created_at, id) < ($6, $7) ORDER BY created_at DESC, id DESC LIMIT 21",
			expectedArgs: []interface{}{
				"dispute_1", "dispute_2", dispute.DisputeEventWebhookOpened, testTime.UTC(), testTime.Add(24 * time.Hour).UTC(), testTime.UTC(), "event_123",
			},
		},
		{
			name: "query with empty dispute IDs",
			query: dispute.DisputeEventQuery{
				DisputeIDs: []string{},
				Limit:      10,
			},
			expectedSQL:  "SELECT id, dispute_id, kind, provider_event_id, data, created_at FROM dispute_events ORDER BY created_at DESC, id DESC LIMIT 11",
			expectedArgs: nil,
		},
		{
			name: "query with only time_from",
			query: dispute.DisputeEventQuery{
				DisputeIDs: []string{"dispute_1"},
				TimeFrom:   &testTime,
				Limit:      10,
			},
			expectedSQL: "SELECT id, dispute_id, kind, provider_event_id, data, created_at FROM dispute_events WHERE dispute_id IN ($1) AND created_at >= $2 ORDER BY created_at DESC, id DESC LIMIT 11",
			expectedArgs: []interface{}{
				"dispute_1", testTime.UTC(),
			},
		},
		{
			name: "query with only time_to",
			query: dispute.DisputeEventQuery{
				DisputeIDs: []string{"dispute_1"},
				TimeTo:     &testTime,
				Limit:      10,
			},
			expectedSQL: "SELECT id, dispute_id, kind, provider_event_id, data, created_at FROM dispute_events WHERE dispute_id IN ($1) AND created_at < $2 ORDER BY created_at DESC, id DESC LIMIT 11",
			expectedArgs: []interface{}{
				"dispute_1", testTime.UTC(),
			},
		},
		{
			name: "query with invalid cursor",
			query: dispute.DisputeEventQuery{
				DisputeIDs: []string{"dispute_1"},
				Cursor:     "invalid-cursor",
				Limit:      10,
			},
			shouldError:   true,
			errorContains: "decode cursor",
		},
		{
			name: "query with ascending sort no cursor",
			query: dispute.DisputeEventQuery{
				DisputeIDs: []string{"dispute_1"},
				SortAsc:    true,
				Limit:      10,
			},
			expectedSQL: "SELECT id, dispute_id, kind, provider_event_id, data, created_at FROM dispute_events WHERE dispute_id IN ($1) ORDER BY created_at ASC, id ASC LIMIT 11",
			expectedArgs: []interface{}{
				"dispute_1",
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			sql, args, err := repo.buildDisputeEventPageQuery(tt.query)

			if tt.shouldError {
				require.Error(t, err)
				if tt.errorContains != "" {
					assert.Contains(t, err.Error(), tt.errorContains)
				}
				return
			}

			require.NoError(t, err)
			assert.Equal(t, tt.expectedSQL, sql)
			assert.Equal(t, tt.expectedArgs, args)
		})
	}
}
