package dispute_repo

import (
	"TestTaskJustPay/internal/domain/dispute"
	"TestTaskJustPay/internal/domain/gateway"
	"TestTaskJustPay/pkg/postgres"
	"context"
	"encoding/json"
	"fmt"
	"testing"
	"time"

	"github.com/Masterminds/squirrel"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/pashagolub/pgxmock/v4"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// testPgDisputeRepo wraps the mock pool to implement the transaction testing
type testPgDisputeRepo struct {
	repo
	pool pgxmock.PgxPoolIface
	pg   *postgres.Postgres
}

func (r *testPgDisputeRepo) InTransaction(ctx context.Context, fn func(repo dispute.TxDisputeRepo) error) error {
	tx, err := r.pool.Begin(ctx)
	if err != nil {
		return fmt.Errorf("begin transaction: %w", err)
	}

	txRepo := &repo{db: tx, builder: r.pg.Builder}

	if err := fn(txRepo); err != nil {
		tx.Rollback(ctx)
		return err
	}

	if err := tx.Commit(ctx); err != nil {
		return fmt.Errorf("commit transaction: %w", err)
	}

	return nil
}

func TestGetDisputeByID(t *testing.T) {
	mock, err := pgxmock.NewPool()
	require.NoError(t, err)
	defer mock.Close()

	repo := &repo{db: mock, builder: squirrel.StatementBuilder.PlaceholderFormat(squirrel.Dollar)}
	ctx := context.Background()

	t.Run("should return dispute with basic query", func(t *testing.T) {
		disputeID := "dispute-1"
		expectedTime := time.Now()

		rows := mock.NewRows([]string{"id", "order_id", "status", "reason", "amount", "currency", "opened_at", "evidence_due_at", "submitted_at", "closed_at"}).
			AddRow(disputeID, "order-1", "open", "fraud", 100.50, "USD", expectedTime, nil, nil, nil)

		mock.ExpectQuery(`SELECT id, order_id, status, reason, amount, currency, opened_at, evidence_due_at, submitted_at, closed_at FROM disputes WHERE id = \$1`).
			WithArgs(disputeID).
			WillReturnRows(rows)

		result, err := repo.GetDisputeByID(ctx, disputeID)

		require.NoError(t, err)
		require.NotNil(t, result)
		assert.Equal(t, disputeID, result.ID)
		assert.Equal(t, "order-1", result.OrderID)
		assert.Equal(t, dispute.DisputeOpen, result.Status)
		assert.Equal(t, "fraud", result.Reason)
		assert.Equal(t, 100.50, result.Amount)
		assert.Equal(t, "USD", result.Currency)
		assert.Equal(t, expectedTime, result.OpenedAt)
		assert.Nil(t, result.EvidenceDueAt)
		assert.Nil(t, result.SubmittedAt)
		assert.Nil(t, result.ClosedAt)
	})

	t.Run("should return nil when dispute not found", func(t *testing.T) {
		disputeID := "nonexistent"

		mock.ExpectQuery(`SELECT id, order_id, status, reason, amount, currency, opened_at, evidence_due_at, submitted_at, closed_at FROM disputes WHERE id = \$1`).
			WithArgs(disputeID).
			WillReturnRows(pgxmock.NewRows([]string{"id", "order_id", "status", "reason", "amount", "currency", "opened_at", "evidence_due_at", "submitted_at", "closed_at"}))

		result, err := repo.GetDisputeByID(ctx, disputeID)

		require.NoError(t, err)
		assert.Nil(t, result)
	})

	t.Run("should handle database error", func(t *testing.T) {
		disputeID := "dispute-1"

		mock.ExpectQuery(`SELECT id, order_id, status, reason, amount, currency, opened_at, evidence_due_at, submitted_at, closed_at FROM disputes WHERE id = \$1`).
			WithArgs(disputeID).
			WillReturnError(assert.AnError)

		result, err := repo.GetDisputeByID(ctx, disputeID)

		require.Error(t, err)
		assert.Nil(t, result)
		assert.Contains(t, err.Error(), "query dispute by id")
	})
}

func TestGetDisputeByOrderID(t *testing.T) {
	mock, err := pgxmock.NewPool()
	require.NoError(t, err)
	defer mock.Close()

	repo := &repo{db: mock, builder: squirrel.StatementBuilder.PlaceholderFormat(squirrel.Dollar)}
	ctx := context.Background()

	t.Run("should return dispute by order ID", func(t *testing.T) {
		orderID := "order-1"
		expectedTime := time.Now()
		evidenceTime := expectedTime.Add(7 * 24 * time.Hour)

		rows := mock.NewRows([]string{"id", "order_id", "status", "reason", "amount", "currency", "opened_at", "evidence_due_at", "submitted_at", "closed_at"}).
			AddRow("dispute-1", orderID, "open", "fraud", 100.50, "USD", expectedTime, evidenceTime, nil, nil)

		mock.ExpectQuery(`SELECT id, order_id, status, reason, amount, currency, opened_at, evidence_due_at, submitted_at, closed_at FROM disputes WHERE order_id = \$1`).
			WithArgs(orderID).
			WillReturnRows(rows)

		result, err := repo.GetDisputeByOrderID(ctx, orderID)

		require.NoError(t, err)
		require.NotNil(t, result)
		assert.Equal(t, "dispute-1", result.ID)
		assert.Equal(t, orderID, result.OrderID)
		assert.NotNil(t, result.EvidenceDueAt)
		assert.Equal(t, evidenceTime, *result.EvidenceDueAt)
	})
}

func TestCreateDispute(t *testing.T) {
	mock, err := pgxmock.NewPool()
	require.NoError(t, err)
	defer mock.Close()

	repo := &repo{db: mock, builder: squirrel.StatementBuilder.PlaceholderFormat(squirrel.Dollar)}
	ctx := context.Background()

	t.Run("should create dispute successfully", func(t *testing.T) {
		openedAt := time.Now()
		evidenceDueAt := openedAt.Add(7 * 24 * time.Hour)

		newDispute := dispute.NewDispute{
			Status: dispute.DisputeOpen,
			DisputeInfo: dispute.DisputeInfo{
				OrderID: "order-1",
				Reason:  "fraud",
				Money: dispute.Money{
					Amount:   100.50,
					Currency: "USD",
				},
				OpenedAt:      openedAt,
				EvidenceDueAt: &evidenceDueAt,
			},
		}

		mock.ExpectExec(`INSERT INTO disputes \(id,order_id,status,reason,amount,currency,opened_at,evidence_due_at,submitted_at,closed_at\) VALUES \(\$1,\$2,\$3,\$4,\$5,\$6,\$7,\$8,\$9,\$10\)`).
			WithArgs(pgxmock.AnyArg(), "order-1", dispute.DisputeOpen, "fraud", 100.50, "USD", openedAt, &evidenceDueAt, (*time.Time)(nil), (*time.Time)(nil)).
			WillReturnResult(pgxmock.NewResult("INSERT", 1))

		result, err := repo.CreateDispute(ctx, newDispute)

		require.NoError(t, err)
		require.NotNil(t, result)
		assert.NotEmpty(t, result.ID)
		assert.Equal(t, dispute.DisputeOpen, result.Status)
		assert.Equal(t, "order-1", result.OrderID)
		assert.Equal(t, "fraud", result.Reason)
		assert.Equal(t, 100.50, result.Amount)
		assert.Equal(t, "USD", result.Currency)
	})

	t.Run("should handle database error", func(t *testing.T) {
		newDispute := dispute.NewDispute{
			Status: dispute.DisputeOpen,
			DisputeInfo: dispute.DisputeInfo{
				OrderID: "order-1",
				Reason:  "fraud",
				Money: dispute.Money{
					Amount:   100.50,
					Currency: "USD",
				},
				OpenedAt: time.Now(),
			},
		}

		mock.ExpectExec(`INSERT INTO disputes \(id,order_id,status,reason,amount,currency,opened_at,evidence_due_at,submitted_at,closed_at\) VALUES \(\$1,\$2,\$3,\$4,\$5,\$6,\$7,\$8,\$9,\$10\)`).
			WillReturnError(assert.AnError)

		result, err := repo.CreateDispute(ctx, newDispute)

		require.Error(t, err)
		assert.Nil(t, result)
		assert.Contains(t, err.Error(), "create dispute")
	})
}

func TestUpdateDispute(t *testing.T) {
	mock, err := pgxmock.NewPool()
	require.NoError(t, err)
	defer mock.Close()

	repo := &repo{db: mock, builder: squirrel.StatementBuilder.PlaceholderFormat(squirrel.Dollar)}
	ctx := context.Background()

	t.Run("should update dispute successfully", func(t *testing.T) {
		closedAt := time.Now()
		disputeToUpdate := dispute.Dispute{
			ID:     "dispute-1",
			Status: dispute.DisputeWon,
			DisputeInfo: dispute.DisputeInfo{
				OrderID: "order-1",
				Reason:  "fraud",
				Money: dispute.Money{
					Amount:   100.50,
					Currency: "USD",
				},
				OpenedAt: time.Now().Add(-7 * 24 * time.Hour),
				ClosedAt: &closedAt,
			},
		}

		mock.ExpectExec(`UPDATE disputes SET status = \$1, reason = \$2, amount = \$3, currency = \$4, opened_at = \$5, evidence_due_at = \$6, submitted_at = \$7, closed_at = \$8 WHERE id = \$9`).
			WithArgs(dispute.DisputeWon, "fraud", 100.50, "USD", disputeToUpdate.OpenedAt, (*time.Time)(nil), (*time.Time)(nil), &closedAt, "dispute-1").
			WillReturnResult(pgxmock.NewResult("UPDATE", 1))

		err := repo.UpdateDispute(ctx, disputeToUpdate)

		require.NoError(t, err)
	})

	t.Run("should handle database error", func(t *testing.T) {
		disputeToUpdate := dispute.Dispute{
			ID:     "dispute-1",
			Status: dispute.DisputeWon,
			DisputeInfo: dispute.DisputeInfo{
				OrderID: "order-1",
				Reason:  "fraud",
				Money: dispute.Money{
					Amount:   100.50,
					Currency: "USD",
				},
				OpenedAt: time.Now(),
			},
		}

		mock.ExpectExec(`UPDATE disputes SET status = \$1, reason = \$2, amount = \$3, currency = \$4, opened_at = \$5, evidence_due_at = \$6, submitted_at = \$7, closed_at = \$8 WHERE id = \$9`).
			WillReturnError(assert.AnError)

		err := repo.UpdateDispute(ctx, disputeToUpdate)

		require.Error(t, err)
		assert.Contains(t, err.Error(), "update dispute")
	})
}

func TestCreateDisputeEvent(t *testing.T) {
	mock, err := pgxmock.NewPool()
	require.NoError(t, err)
	defer mock.Close()

	repo := &repo{db: mock, builder: squirrel.StatementBuilder.PlaceholderFormat(squirrel.Dollar)}
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

func TestInTransaction(t *testing.T) {
	mock, err := pgxmock.NewPool()
	require.NoError(t, err)
	defer mock.Close()

	pg := &postgres.Postgres{
		Builder: squirrel.StatementBuilder.PlaceholderFormat(squirrel.Dollar),
	}
	pgRepo := &testPgDisputeRepo{
		repo: repo{db: mock, builder: squirrel.StatementBuilder.PlaceholderFormat(squirrel.Dollar)},
		pool: mock,
		pg:   pg,
	}
	ctx := context.Background()

	t.Run("should execute function in transaction successfully", func(t *testing.T) {
		mock.ExpectBegin()
		mock.ExpectCommit()

		executed := false
		err := pgRepo.InTransaction(ctx, func(repo dispute.TxDisputeRepo) error {
			executed = true
			assert.NotNil(t, repo)
			return nil
		})

		require.NoError(t, err)
		assert.True(t, executed)
	})

	t.Run("should rollback transaction on function error", func(t *testing.T) {
		mock.ExpectBegin()
		mock.ExpectRollback()

		expectedErr := assert.AnError
		err := pgRepo.InTransaction(ctx, func(repo dispute.TxDisputeRepo) error {
			return expectedErr
		})

		require.Error(t, err)
		assert.Equal(t, expectedErr, err)
	})

	t.Run("should handle begin transaction error", func(t *testing.T) {
		mock.ExpectBegin().WillReturnError(assert.AnError)

		err := pgRepo.InTransaction(ctx, func(repo dispute.TxDisputeRepo) error {
			return nil
		})

		require.Error(t, err)
		assert.Contains(t, err.Error(), "begin transaction")
	})

	t.Run("should handle commit error", func(t *testing.T) {
		mock.ExpectBegin()
		mock.ExpectCommit().WillReturnError(assert.AnError)

		err := pgRepo.InTransaction(ctx, func(repo dispute.TxDisputeRepo) error {
			return nil
		})

		require.Error(t, err)
		assert.Contains(t, err.Error(), "commit transaction")
	})

	t.Run("should handle rollback error after function error", func(t *testing.T) {
		mock.ExpectBegin()
		mock.ExpectRollback().WillReturnError(assert.AnError)

		functionErr := assert.AnError
		err := pgRepo.InTransaction(ctx, func(repo dispute.TxDisputeRepo) error {
			return functionErr
		})

		require.Error(t, err)
		// Should return the original function error, not the rollback error
		assert.Equal(t, functionErr, err)
	})
}

func TestUpsertEvidence(t *testing.T) {
	t.Run("should upsert evidence successfully", func(t *testing.T) {
		mock, err := pgxmock.NewPool()
		require.NoError(t, err)
		defer mock.Close()

		repo := &repo{db: mock, builder: squirrel.StatementBuilder.PlaceholderFormat(squirrel.Dollar)}
		ctx := context.Background()

		disputeID := "dispute-1"
		upsert := dispute.EvidenceUpsert{
			Evidence: gateway.Evidence{
				Fields: map[string]string{
					"transaction_receipt":    "receipt_123",
					"customer_communication": "email_456",
				},
				Files: []gateway.EvidenceFile{
					{
						FileID:      "file-1",
						Name:        "receipt.pdf",
						ContentType: "application/pdf",
						Size:        1024,
					},
					{
						FileID:      "file-2",
						Name:        "communication.txt",
						ContentType: "text/plain",
						Size:        512,
					},
				},
			},
		}

		expectedFieldsJSON := []byte(`{"customer_communication":"email_456","transaction_receipt":"receipt_123"}`)
		expectedFilesJSON := []byte(`[{"file_id":"file-1","name":"receipt.pdf","content_type":"application/pdf","size":1024},{"file_id":"file-2","name":"communication.txt","content_type":"text/plain","size":512}]`)

		mock.ExpectExec(`INSERT INTO evidence \(dispute_id,fields,files,updated_at\) VALUES \(\$1,\$2,\$3,\$4\) ON CONFLICT \(dispute_id\) DO UPDATE SET fields = EXCLUDED\.fields, files = EXCLUDED\.files, updated_at = EXCLUDED\.updated_at`).
			WithArgs(disputeID, expectedFieldsJSON, expectedFilesJSON, pgxmock.AnyArg()).
			WillReturnResult(pgxmock.NewResult("INSERT", 1))

		result, err := repo.UpsertEvidence(ctx, disputeID, upsert)

		require.NoError(t, err)
		require.NotNil(t, result)
		assert.Equal(t, disputeID, result.DisputeID)
		assert.Equal(t, upsert.Fields, result.Fields)
		assert.Equal(t, upsert.Files, result.Files)
		assert.NotZero(t, result.UpdatedAt)
	})

	t.Run("should handle empty fields and files", func(t *testing.T) {
		mock, err := pgxmock.NewPool()
		require.NoError(t, err)
		defer mock.Close()

		repo := &repo{db: mock, builder: squirrel.StatementBuilder.PlaceholderFormat(squirrel.Dollar)}
		ctx := context.Background()

		disputeID := "dispute-2"
		upsert := dispute.EvidenceUpsert{
			Evidence: gateway.Evidence{
				Fields: map[string]string{},
				Files:  []gateway.EvidenceFile{},
			},
		}

		expectedFieldsJSON := []byte(`{}`)
		expectedFilesJSON := []byte(`[]`)

		mock.ExpectExec(`INSERT INTO evidence \(dispute_id,fields,files,updated_at\) VALUES \(\$1,\$2,\$3,\$4\) ON CONFLICT \(dispute_id\) DO UPDATE SET fields = EXCLUDED\.fields, files = EXCLUDED\.files, updated_at = EXCLUDED\.updated_at`).
			WithArgs(disputeID, expectedFieldsJSON, expectedFilesJSON, pgxmock.AnyArg()).
			WillReturnResult(pgxmock.NewResult("INSERT", 1))

		result, err := repo.UpsertEvidence(ctx, disputeID, upsert)

		require.NoError(t, err)
		require.NotNil(t, result)
		assert.Equal(t, disputeID, result.DisputeID)
		assert.Empty(t, result.Fields)
		assert.Empty(t, result.Files)
	})

	t.Run("should handle database error", func(t *testing.T) {
		mock, err := pgxmock.NewPool()
		require.NoError(t, err)
		defer mock.Close()

		repo := &repo{db: mock, builder: squirrel.StatementBuilder.PlaceholderFormat(squirrel.Dollar)}
		ctx := context.Background()

		disputeID := "dispute-3"
		upsert := dispute.EvidenceUpsert{
			Evidence: gateway.Evidence{
				Fields: map[string]string{"key": "value"},
				Files:  []gateway.EvidenceFile{},
			},
		}

		mock.ExpectExec(`INSERT INTO evidence \(dispute_id,fields,files,updated_at\) VALUES \(\$1,\$2,\$3,\$4\) ON CONFLICT \(dispute_id\) DO UPDATE SET fields = EXCLUDED\.fields, files = EXCLUDED\.files, updated_at = EXCLUDED\.updated_at`).
			WillReturnError(assert.AnError)

		result, err := repo.UpsertEvidence(ctx, disputeID, upsert)

		require.Error(t, err)
		assert.Nil(t, result)
		assert.Contains(t, err.Error(), "upsert evidence")
	})

	t.Run("should handle nil fields map", func(t *testing.T) {
		mock, err := pgxmock.NewPool()
		require.NoError(t, err)
		defer mock.Close()

		repo := &repo{db: mock, builder: squirrel.StatementBuilder.PlaceholderFormat(squirrel.Dollar)}
		ctx := context.Background()

		disputeID := "dispute-4"
		upsert := dispute.EvidenceUpsert{
			Evidence: gateway.Evidence{
				Fields: nil,
				Files:  []gateway.EvidenceFile{},
			},
		}

		expectedFieldsJSON := []byte(`null`)
		expectedFilesJSON := []byte(`[]`)

		mock.ExpectExec(`INSERT INTO evidence \(dispute_id,fields,files,updated_at\) VALUES \(\$1,\$2,\$3,\$4\) ON CONFLICT \(dispute_id\) DO UPDATE SET fields = EXCLUDED\.fields, files = EXCLUDED\.files, updated_at = EXCLUDED\.updated_at`).
			WithArgs(disputeID, expectedFieldsJSON, expectedFilesJSON, pgxmock.AnyArg()).
			WillReturnResult(pgxmock.NewResult("INSERT", 1))

		result, err := repo.UpsertEvidence(ctx, disputeID, upsert)

		require.NoError(t, err)
		require.NotNil(t, result)
		assert.Equal(t, disputeID, result.DisputeID)
		assert.Nil(t, result.Fields)
		assert.Empty(t, result.Files)
	})

	t.Run("should handle nil files slice", func(t *testing.T) {
		mock, err := pgxmock.NewPool()
		require.NoError(t, err)
		defer mock.Close()

		repo := &repo{db: mock, builder: squirrel.StatementBuilder.PlaceholderFormat(squirrel.Dollar)}
		ctx := context.Background()

		disputeID := "dispute-5"
		upsert := dispute.EvidenceUpsert{
			Evidence: gateway.Evidence{
				Fields: map[string]string{"key": "value"},
				Files:  nil,
			},
		}

		expectedFieldsJSON := []byte(`{"key":"value"}`)
		expectedFilesJSON := []byte(`null`)

		mock.ExpectExec(`INSERT INTO evidence \(dispute_id,fields,files,updated_at\) VALUES \(\$1,\$2,\$3,\$4\) ON CONFLICT \(dispute_id\) DO UPDATE SET fields = EXCLUDED\.fields, files = EXCLUDED\.files, updated_at = EXCLUDED\.updated_at`).
			WithArgs(disputeID, expectedFieldsJSON, expectedFilesJSON, pgxmock.AnyArg()).
			WillReturnResult(pgxmock.NewResult("INSERT", 1))

		result, err := repo.UpsertEvidence(ctx, disputeID, upsert)

		require.NoError(t, err)
		require.NotNil(t, result)
		assert.Equal(t, disputeID, result.DisputeID)
		assert.Equal(t, map[string]string{"key": "value"}, result.Fields)
		assert.Nil(t, result.Files)
	})
}

func TestGetDisputeEvents(t *testing.T) {
	t.Run("should return dispute events with basic query", func(t *testing.T) {
		mock, err := pgxmock.NewPool()
		require.NoError(t, err)
		defer mock.Close()

		repo := &repo{db: mock, builder: squirrel.StatementBuilder.PlaceholderFormat(squirrel.Dollar)}
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

		repo := &repo{db: mock, builder: squirrel.StatementBuilder.PlaceholderFormat(squirrel.Dollar)}
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

		repo := &repo{db: mock, builder: squirrel.StatementBuilder.PlaceholderFormat(squirrel.Dollar)}
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

		repo := &repo{db: mock, builder: squirrel.StatementBuilder.PlaceholderFormat(squirrel.Dollar)}
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

		repo := &repo{db: mock, builder: squirrel.StatementBuilder.PlaceholderFormat(squirrel.Dollar)}
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

		repo := &repo{db: mock, builder: squirrel.StatementBuilder.PlaceholderFormat(squirrel.Dollar)}
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

		repo := &repo{db: mock, builder: squirrel.StatementBuilder.PlaceholderFormat(squirrel.Dollar)}
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

func TestGetEvidence(t *testing.T) {
	t.Run("should return evidence for dispute", func(t *testing.T) {
		mock, err := pgxmock.NewPool()
		require.NoError(t, err)
		defer mock.Close()

		repo := &repo{db: mock, builder: squirrel.StatementBuilder.PlaceholderFormat(squirrel.Dollar)}
		ctx := context.Background()

		disputeID := "dispute-1"
		updatedAt := time.Now()
		fieldsJSON := `{"transaction_receipt":"receipt_123","customer_communication":"email_456"}`
		filesJSON := `[{"file_id":"file-1","name":"receipt.pdf","content_type":"application/pdf","size":1024}]`

		rows := mock.NewRows([]string{"dispute_id", "fields", "files", "updated_at"}).
			AddRow(disputeID, fieldsJSON, filesJSON, updatedAt)

		mock.ExpectQuery(`SELECT dispute_id, fields, files, updated_at FROM evidence WHERE dispute_id = \$1`).
			WithArgs(disputeID).
			WillReturnRows(rows)

		result, err := repo.GetEvidence(ctx, disputeID)

		require.NoError(t, err)
		require.NotNil(t, result)
		assert.Equal(t, disputeID, result.DisputeID)
		assert.Equal(t, "receipt_123", result.Fields["transaction_receipt"])
		assert.Equal(t, "email_456", result.Fields["customer_communication"])
		assert.Len(t, result.Files, 1)
		assert.Equal(t, "file-1", result.Files[0].FileID)
		assert.Equal(t, "receipt.pdf", result.Files[0].Name)
		assert.Equal(t, "application/pdf", result.Files[0].ContentType)
		assert.Equal(t, 1024, result.Files[0].Size)
		assert.Equal(t, updatedAt, result.UpdatedAt)
	})

	t.Run("should return nil when evidence not found", func(t *testing.T) {
		mock, err := pgxmock.NewPool()
		require.NoError(t, err)
		defer mock.Close()

		repo := &repo{db: mock, builder: squirrel.StatementBuilder.PlaceholderFormat(squirrel.Dollar)}
		ctx := context.Background()

		disputeID := "nonexistent"

		mock.ExpectQuery(`SELECT dispute_id, fields, files, updated_at FROM evidence WHERE dispute_id = \$1`).
			WithArgs(disputeID).
			WillReturnRows(pgxmock.NewRows([]string{"dispute_id", "fields", "files", "updated_at"}))

		result, err := repo.GetEvidence(ctx, disputeID)

		require.NoError(t, err)
		assert.Nil(t, result)
	})

	t.Run("should handle database error", func(t *testing.T) {
		mock, err := pgxmock.NewPool()
		require.NoError(t, err)
		defer mock.Close()

		repo := &repo{db: mock, builder: squirrel.StatementBuilder.PlaceholderFormat(squirrel.Dollar)}
		ctx := context.Background()

		disputeID := "dispute-1"

		mock.ExpectQuery(`SELECT dispute_id, fields, files, updated_at FROM evidence WHERE dispute_id = \$1`).
			WithArgs(disputeID).
			WillReturnError(assert.AnError)

		result, err := repo.GetEvidence(ctx, disputeID)

		require.Error(t, err)
		assert.Nil(t, result)
		assert.Contains(t, err.Error(), "query evidence")
	})

	t.Run("should handle empty fields and files", func(t *testing.T) {
		mock, err := pgxmock.NewPool()
		require.NoError(t, err)
		defer mock.Close()

		repo := &repo{db: mock, builder: squirrel.StatementBuilder.PlaceholderFormat(squirrel.Dollar)}
		ctx := context.Background()

		disputeID := "dispute-2"
		updatedAt := time.Now()
		fieldsJSON := `{}`
		filesJSON := `[]`

		rows := mock.NewRows([]string{"dispute_id", "fields", "files", "updated_at"}).
			AddRow(disputeID, fieldsJSON, filesJSON, updatedAt)

		mock.ExpectQuery(`SELECT dispute_id, fields, files, updated_at FROM evidence WHERE dispute_id = \$1`).
			WithArgs(disputeID).
			WillReturnRows(rows)

		result, err := repo.GetEvidence(ctx, disputeID)

		require.NoError(t, err)
		require.NotNil(t, result)
		assert.Equal(t, disputeID, result.DisputeID)
		assert.Empty(t, result.Fields)
		assert.Empty(t, result.Files)
		assert.Equal(t, updatedAt, result.UpdatedAt)
	})

	t.Run("should handle null fields and files", func(t *testing.T) {
		mock, err := pgxmock.NewPool()
		require.NoError(t, err)
		defer mock.Close()

		repo := &repo{db: mock, builder: squirrel.StatementBuilder.PlaceholderFormat(squirrel.Dollar)}
		ctx := context.Background()

		disputeID := "dispute-3"
		updatedAt := time.Now()
		fieldsJSON := `null`
		filesJSON := `null`

		rows := mock.NewRows([]string{"dispute_id", "fields", "files", "updated_at"}).
			AddRow(disputeID, fieldsJSON, filesJSON, updatedAt)

		mock.ExpectQuery(`SELECT dispute_id, fields, files, updated_at FROM evidence WHERE dispute_id = \$1`).
			WithArgs(disputeID).
			WillReturnRows(rows)

		result, err := repo.GetEvidence(ctx, disputeID)

		require.NoError(t, err)
		require.NotNil(t, result)
		assert.Equal(t, disputeID, result.DisputeID)
		assert.Nil(t, result.Fields)
		assert.Nil(t, result.Files)
		assert.Equal(t, updatedAt, result.UpdatedAt)
	})

	t.Run("should handle scan error", func(t *testing.T) {
		mock, err := pgxmock.NewPool()
		require.NoError(t, err)
		defer mock.Close()

		repo := &repo{db: mock, builder: squirrel.StatementBuilder.PlaceholderFormat(squirrel.Dollar)}
		ctx := context.Background()

		disputeID := "dispute-1"

		rows := mock.NewRows([]string{"dispute_id", "fields", "files", "updated_at"}).
			AddRow(disputeID, "invalid-json", "{}", "invalid-time")

		mock.ExpectQuery(`SELECT dispute_id, fields, files, updated_at FROM evidence WHERE dispute_id = \$1`).
			WithArgs(disputeID).
			WillReturnRows(rows)

		result, err := repo.GetEvidence(ctx, disputeID)

		require.Error(t, err)
		assert.Nil(t, result)
		assert.Contains(t, err.Error(), "scan evidence row")
	})

	t.Run("should handle invalid fields JSON", func(t *testing.T) {
		mock, err := pgxmock.NewPool()
		require.NoError(t, err)
		defer mock.Close()

		repo := &repo{db: mock, builder: squirrel.StatementBuilder.PlaceholderFormat(squirrel.Dollar)}
		ctx := context.Background()

		disputeID := "dispute-1"
		updatedAt := time.Now()
		fieldsJSON := `{invalid-json}`
		filesJSON := `[]`

		rows := mock.NewRows([]string{"dispute_id", "fields", "files", "updated_at"}).
			AddRow(disputeID, fieldsJSON, filesJSON, updatedAt)

		mock.ExpectQuery(`SELECT dispute_id, fields, files, updated_at FROM evidence WHERE dispute_id = \$1`).
			WithArgs(disputeID).
			WillReturnRows(rows)

		result, err := repo.GetEvidence(ctx, disputeID)

		require.Error(t, err)
		assert.Nil(t, result)
		assert.Contains(t, err.Error(), "unmarshal fields")
	})

	t.Run("should handle invalid files JSON", func(t *testing.T) {
		mock, err := pgxmock.NewPool()
		require.NoError(t, err)
		defer mock.Close()

		repo := &repo{db: mock, builder: squirrel.StatementBuilder.PlaceholderFormat(squirrel.Dollar)}
		ctx := context.Background()

		disputeID := "dispute-1"
		updatedAt := time.Now()
		fieldsJSON := `{}`
		filesJSON := `[invalid-json]`

		rows := mock.NewRows([]string{"dispute_id", "fields", "files", "updated_at"}).
			AddRow(disputeID, fieldsJSON, filesJSON, updatedAt)

		mock.ExpectQuery(`SELECT dispute_id, fields, files, updated_at FROM evidence WHERE dispute_id = \$1`).
			WithArgs(disputeID).
			WillReturnRows(rows)

		result, err := repo.GetEvidence(ctx, disputeID)

		require.Error(t, err)
		assert.Nil(t, result)
		assert.Contains(t, err.Error(), "unmarshal files")
	})
}
