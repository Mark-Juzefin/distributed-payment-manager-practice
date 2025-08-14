package dispute_repo

import (
	"TestTaskJustPay/internal/domain/dispute"
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
