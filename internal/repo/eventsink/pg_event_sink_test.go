package eventsink

import (
	"TestTaskJustPay/internal/domain/dispute"
	"context"
	"encoding/json"
	"testing"
	"time"

	"github.com/Masterminds/squirrel"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/pashagolub/pgxmock/v4"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestCreateDisputeEvent(t *testing.T) {
	mock, err := pgxmock.NewPool()
	require.NoError(t, err)
	defer mock.Close()

	repo := &PgEventRepo{db: mock, builder: squirrel.StatementBuilder.PlaceholderFormat(squirrel.Dollar)}
	ctx := context.Background()

	t.Run("should create dispute event successfully", func(t *testing.T) {
		createdAt := time.Now()
		event := dispute.NewDisputeEvent{
			DisputeID:       "dispute-1",
			Kind:            dispute.DisputeEventWebhookOpened,
			ProviderEventID: "provider-event-1",
			Data:            json.RawMessage(`{"test": "data"}`),
			CreatedAt:       createdAt,
		}

		mock.ExpectExec(`INSERT INTO dispute_events \(id,dispute_id,kind,provider_event_id,data,created_at\) VALUES \(\$1,\$2,\$3,\$4,\$5,\$6\)`).
			WithArgs(pgxmock.AnyArg(), "dispute-1", dispute.DisputeEventWebhookOpened, "provider-event-1", json.RawMessage(`{"test": "data"}`), createdAt).
			WillReturnResult(pgxmock.NewResult("INSERT", 1))

		err := repo.CreateDisputeEvent(ctx, event)

		require.NoError(t, err)
	})

	t.Run("should handle unique violation error", func(t *testing.T) {
		event := dispute.NewDisputeEvent{
			DisputeID:       "dispute-1",
			Kind:            dispute.DisputeEventWebhookOpened,
			ProviderEventID: "provider-event-1",
			Data:            json.RawMessage(`{"test": "data"}`),
			CreatedAt:       time.Now(),
		}

		pgErr := &pgconn.PgError{
			Code: "23505", // unique_violation
		}

		mock.ExpectExec(`INSERT INTO dispute_events \(id,dispute_id,kind,provider_event_id,data,created_at\) VALUES \(\$1,\$2,\$3,\$4,\$5,\$6\)`).
			WithArgs(pgxmock.AnyArg(), "dispute-1", dispute.DisputeEventWebhookOpened, "provider-event-1", json.RawMessage(`{"test": "data"}`), event.CreatedAt).
			WillReturnError(pgErr)

		err := repo.CreateDisputeEvent(ctx, event)

		require.Error(t, err)
		assert.Contains(t, err.Error(), "dispute event already exists")
	})

	t.Run("should handle other database errors", func(t *testing.T) {
		event := dispute.NewDisputeEvent{
			DisputeID:       "dispute-1",
			Kind:            dispute.DisputeEventWebhookOpened,
			ProviderEventID: "provider-event-1",
			Data:            json.RawMessage(`{"test": "data"}`),
			CreatedAt:       time.Now(),
		}

		mock.ExpectExec(`INSERT INTO dispute_events \(id,dispute_id,kind,provider_event_id,data,created_at\) VALUES \(\$1,\$2,\$3,\$4,\$5,\$6\)`).
			WillReturnError(assert.AnError)

		err := repo.CreateDisputeEvent(ctx, event)

		require.Error(t, err)
		assert.Contains(t, err.Error(), "create dispute event")
	})
}

func TestGetDisputeEvents(t *testing.T) {
	t.Run("should return dispute events with basic query", func(t *testing.T) {
		mock, err := pgxmock.NewPool()
		require.NoError(t, err)
		defer mock.Close()

		repo := &PgEventRepo{db: mock, builder: squirrel.StatementBuilder.PlaceholderFormat(squirrel.Dollar)}
		ctx := context.Background()

		query := &dispute.DisputeEventQuery{
			DisputeIDs: []string{"dispute-1"},
		}

		createdAt := time.Now()
		rows := mock.NewRows([]string{"id", "dispute_id", "kind", "provider_event_id", "data", "created_at"}).
			AddRow("event-1", "dispute-1", "webhook_opened", "ev-1", json.RawMessage(`{"test":"data1"}`), createdAt).
			AddRow("event-2", "dispute-1", "evidence_added", "ev-2", json.RawMessage(`{"test":"data2"}`), createdAt.Add(time.Hour))

		mock.ExpectQuery(`SELECT id, dispute_id, kind, provider_event_id, data, created_at FROM dispute_events WHERE dispute_id IN \(\$1\) ORDER BY created_at DESC`).
			WithArgs("dispute-1").
			WillReturnRows(rows)

		result, err := repo.GetDisputeEvents(ctx, query)

		require.NoError(t, err)
		require.Len(t, result, 2)

		assert.Equal(t, "event-1", result[0].EventID)
		assert.Equal(t, "dispute-1", result[0].DisputeID)
		assert.Equal(t, dispute.DisputeEventWebhookOpened, result[0].Kind)
		assert.Equal(t, "ev-1", result[0].ProviderEventID)
		assert.Equal(t, json.RawMessage(`{"test":"data1"}`), result[0].Data)
		assert.Equal(t, createdAt, result[0].CreatedAt)

		assert.Equal(t, "event-2", result[1].EventID)
		assert.Equal(t, dispute.DisputeEventEvidenceAdded, result[1].Kind)
	})

	t.Run("should return events filtered by kinds", func(t *testing.T) {
		mock, err := pgxmock.NewPool()
		require.NoError(t, err)
		defer mock.Close()

		repo := &PgEventRepo{db: mock, builder: squirrel.StatementBuilder.PlaceholderFormat(squirrel.Dollar)}
		ctx := context.Background()

		query := &dispute.DisputeEventQuery{
			DisputeIDs: []string{"dispute-1"},
			Kinds:      []dispute.DisputeEventKind{dispute.DisputeEventWebhookOpened},
		}

		createdAt := time.Now()
		rows := mock.NewRows([]string{"id", "dispute_id", "kind", "provider_event_id", "data", "created_at"}).
			AddRow("event-1", "dispute-1", "webhook_opened", "ev-1", json.RawMessage(`{"test":"data"}`), createdAt)

		mock.ExpectQuery(`SELECT id, dispute_id, kind, provider_event_id, data, created_at FROM dispute_events WHERE dispute_id IN \(\$1\) AND kind IN \(\$2\) ORDER BY created_at DESC`).
			WithArgs("dispute-1", dispute.DisputeEventWebhookOpened).
			WillReturnRows(rows)

		result, err := repo.GetDisputeEvents(ctx, query)

		require.NoError(t, err)
		require.Len(t, result, 1)
		assert.Equal(t, dispute.DisputeEventWebhookOpened, result[0].Kind)
	})

	t.Run("should return events for multiple disputes", func(t *testing.T) {
		mock, err := pgxmock.NewPool()
		require.NoError(t, err)
		defer mock.Close()

		repo := &PgEventRepo{db: mock, builder: squirrel.StatementBuilder.PlaceholderFormat(squirrel.Dollar)}
		ctx := context.Background()

		query := &dispute.DisputeEventQuery{
			DisputeIDs: []string{"dispute-1", "dispute-2"},
		}

		createdAt := time.Now()
		rows := mock.NewRows([]string{"id", "dispute_id", "kind", "provider_event_id", "data", "created_at"}).
			AddRow("event-1", "dispute-1", "webhook_opened", "ev-1", json.RawMessage(`{"test":"data1"}`), createdAt).
			AddRow("event-2", "dispute-2", "webhook_opened", "ev-2", json.RawMessage(`{"test":"data2"}`), createdAt)

		mock.ExpectQuery(`SELECT id, dispute_id, kind, provider_event_id, data, created_at FROM dispute_events WHERE dispute_id IN \(\$1,\$2\) ORDER BY created_at DESC`).
			WithArgs("dispute-1", "dispute-2").
			WillReturnRows(rows)

		result, err := repo.GetDisputeEvents(ctx, query)

		require.NoError(t, err)
		require.Len(t, result, 2)
		assert.Equal(t, "dispute-1", result[0].DisputeID)
		assert.Equal(t, "dispute-2", result[1].DisputeID)
	})

	t.Run("should return empty slice when no events found", func(t *testing.T) {
		mock, err := pgxmock.NewPool()
		require.NoError(t, err)
		defer mock.Close()

		repo := &PgEventRepo{db: mock, builder: squirrel.StatementBuilder.PlaceholderFormat(squirrel.Dollar)}
		ctx := context.Background()

		query := &dispute.DisputeEventQuery{
			DisputeIDs: []string{"nonexistent"},
		}

		mock.ExpectQuery(`SELECT id, dispute_id, kind, provider_event_id, data, created_at FROM dispute_events WHERE dispute_id IN \(\$1\) ORDER BY created_at DESC`).
			WithArgs("nonexistent").
			WillReturnRows(pgxmock.NewRows([]string{"id", "dispute_id", "kind", "provider_event_id", "data", "created_at"}))

		result, err := repo.GetDisputeEvents(ctx, query)

		require.NoError(t, err)
		assert.Empty(t, result)
	})

	t.Run("should handle database error", func(t *testing.T) {
		mock, err := pgxmock.NewPool()
		require.NoError(t, err)
		defer mock.Close()

		repo := &PgEventRepo{db: mock, builder: squirrel.StatementBuilder.PlaceholderFormat(squirrel.Dollar)}
		ctx := context.Background()

		query := &dispute.DisputeEventQuery{
			DisputeIDs: []string{"dispute-1"},
		}

		mock.ExpectQuery(`SELECT id, dispute_id, kind, provider_event_id, data, created_at FROM dispute_events WHERE dispute_id IN \(\$1\) ORDER BY created_at DESC`).
			WithArgs("dispute-1").
			WillReturnError(assert.AnError)

		result, err := repo.GetDisputeEvents(ctx, query)

		require.Error(t, err)
		assert.Nil(t, result)
		assert.Contains(t, err.Error(), "query dispute events")
	})

	t.Run("should handle query without filters", func(t *testing.T) {
		mock, err := pgxmock.NewPool()
		require.NoError(t, err)
		defer mock.Close()

		repo := &PgEventRepo{db: mock, builder: squirrel.StatementBuilder.PlaceholderFormat(squirrel.Dollar)}
		ctx := context.Background()

		query := &dispute.DisputeEventQuery{}

		createdAt := time.Now()
		rows := mock.NewRows([]string{"id", "dispute_id", "kind", "provider_event_id", "data", "created_at"}).
			AddRow("event-1", "dispute-1", "webhook_opened", "ev-1", json.RawMessage(`{"test":"data"}`), createdAt)

		mock.ExpectQuery(`SELECT id, dispute_id, kind, provider_event_id, data, created_at FROM dispute_events ORDER BY created_at DESC`).
			WillReturnRows(rows)

		result, err := repo.GetDisputeEvents(ctx, query)

		require.NoError(t, err)
		require.Len(t, result, 1)
		assert.Equal(t, "event-1", result[0].EventID)
	})

	t.Run("should handle scan error", func(t *testing.T) {
		mock, err := pgxmock.NewPool()
		require.NoError(t, err)
		defer mock.Close()

		repo := &PgEventRepo{db: mock, builder: squirrel.StatementBuilder.PlaceholderFormat(squirrel.Dollar)}
		ctx := context.Background()

		query := &dispute.DisputeEventQuery{
			DisputeIDs: []string{"dispute-1"},
		}

		rows := mock.NewRows([]string{"id", "dispute_id", "kind", "provider_event_id", "data", "created_at"}).
			AddRow("event-1", "dispute-1", "invalid-kind", "ev-1", json.RawMessage(`{"test":"data"}`), "invalid-time")

		mock.ExpectQuery(`SELECT id, dispute_id, kind, provider_event_id, data, created_at FROM dispute_events WHERE dispute_id IN \(\$1\) ORDER BY created_at DESC`).
			WithArgs("dispute-1").
			WillReturnRows(rows)

		result, err := repo.GetDisputeEvents(ctx, query)

		require.Error(t, err)
		assert.Nil(t, result)
		assert.Contains(t, err.Error(), "scan dispute event row")
	})
}
