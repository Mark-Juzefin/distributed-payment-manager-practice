package order_repo

import (
	"TestTaskJustPay/internal/domain/order"
	"TestTaskJustPay/pkg/postgres"
	"context"
	"fmt"
	"testing"
	"time"

	"github.com/Masterminds/squirrel"
	"github.com/google/uuid"
	"github.com/pashagolub/pgxmock/v4"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// testPgOrderRepo wraps the mock pool to implement the transaction testing
type testPgOrderRepo struct {
	repo
	pool pgxmock.PgxPoolIface
	pg   *postgres.Postgres
}

func (r *testPgOrderRepo) InTransaction(ctx context.Context, fn func(repo order.TxOrderRepo) error) error {
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

func TestGetOrders(t *testing.T) {
	mock, err := pgxmock.NewPool()
	require.NoError(t, err)
	defer mock.Close()

	repo := &repo{db: mock, builder: squirrel.StatementBuilder.PlaceholderFormat(squirrel.Dollar)}
	ctx := context.Background()

	t.Run("should return orders with basic query", func(t *testing.T) {
		userId := uuid.NewString()
		expectedTime := time.Now()

		query := &order.OrdersQuery{
			IDs: []string{"order-1", "order-2"},
		}

		rows := mock.NewRows([]string{"id", "user_id", "status", "on_hold", "hold_reason", "created_at", "updated_at"}).
			AddRow("order-1", userId, "created", false, nil, expectedTime, expectedTime).
			AddRow("order-2", userId, "updated", false, nil, expectedTime, expectedTime)

		mock.ExpectQuery(`SELECT id, user_id, status, on_hold, hold_reason, created_at, updated_at FROM orders WHERE id IN \(\$1,\$2\)`).
			WithArgs("order-1", "order-2").
			WillReturnRows(rows)

		result, err := repo.GetOrders(ctx, query)

		require.NoError(t, err)
		assert.Len(t, result, 2)
		assert.Equal(t, "order-1", result[0].OrderId)
		assert.Equal(t, "order-2", result[1].OrderId)
		assert.Equal(t, order.StatusCreated, result[0].Status)
		assert.Equal(t, order.StatusUpdated, result[1].Status)
	})
}

func TestUpdateOrder(t *testing.T) {
	mock, err := pgxmock.NewPool()
	require.NoError(t, err)
	defer mock.Close()

	repo := &repo{db: mock, builder: squirrel.StatementBuilder.PlaceholderFormat(squirrel.Dollar)}
	ctx := context.Background()

	t.Run("should update order successfully", func(t *testing.T) {
		userId := uuid.NewString()
		updatedAt := time.Now()

		event := order.PaymentWebhook{
			OrderId:   "order-1",
			UserId:    userId,
			Status:    order.StatusUpdated,
			UpdatedAt: updatedAt,
		}

		mock.ExpectExec(`UPDATE orders SET status = \$1, updated_at = \$2 WHERE id = \$3`).
			WithArgs(order.StatusUpdated, updatedAt, "order-1").
			WillReturnResult(pgxmock.NewResult("UPDATE", 1))

		err := repo.UpdateOrder(ctx, event)

		require.NoError(t, err)
	})

	t.Run("should handle database error", func(t *testing.T) {
		userId := uuid.NewString()

		event := order.PaymentWebhook{

			OrderId: "order-1",
			UserId:  userId,
			Status:  order.StatusUpdated,
		}

		mock.ExpectExec(`UPDATE order SET status = \$1, updated_at = \$2 WHERE id = \$3`).
			WillReturnError(assert.AnError)

		err := repo.UpdateOrder(ctx, event)

		require.Error(t, err)
		assert.Contains(t, err.Error(), "update order")
	})
}

func TestCreateOrderByEvent(t *testing.T) {
	mock, err := pgxmock.NewPool()
	require.NoError(t, err)
	defer mock.Close()

	repo := &repo{db: mock, builder: squirrel.StatementBuilder.PlaceholderFormat(squirrel.Dollar)}
	ctx := context.Background()

	t.Run("should create order successfully", func(t *testing.T) {
		userId := uuid.NewString()
		createdAt := time.Now()
		updatedAt := time.Now()

		event := order.PaymentWebhook{
			OrderId:   "order-1",
			UserId:    userId,
			Status:    order.StatusCreated,
			CreatedAt: createdAt,
			UpdatedAt: updatedAt,
		}

		mock.ExpectExec(`INSERT INTO orders \(id,user_id,status,created_at,updated_at\) VALUES \(\$1,\$2,\$3,\$4,\$5\)`).
			WithArgs("order-1", userId, order.StatusCreated, createdAt, updatedAt).
			WillReturnResult(pgxmock.NewResult("INSERT", 1))

		err := repo.CreateOrder(ctx, event)

		require.NoError(t, err)
	})

	t.Run("should handle database error", func(t *testing.T) {
		userId := uuid.NewString()

		event := order.PaymentWebhook{
			OrderId: "order-1",
			UserId:  userId,
			Status:  order.StatusCreated,
		}

		mock.ExpectExec(`INSERT INTO orders \(id,user_id,status,created_at,updated_at\) VALUES \(\$1,\$2,\$3,\$4,\$5\)`).
			WillReturnError(assert.AnError)

		err := repo.CreateOrder(ctx, event)

		require.Error(t, err)
		assert.Contains(t, err.Error(), "create order by event")
	})
}

func TestInTransaction(t *testing.T) {
	mock, err := pgxmock.NewPool()
	require.NoError(t, err)
	defer mock.Close()

	pg := &postgres.Postgres{
		Builder: squirrel.StatementBuilder.PlaceholderFormat(squirrel.Dollar),
	}
	// Create a test repository using the mock
	pgRepo := &testPgOrderRepo{
		repo: repo{db: mock, builder: squirrel.StatementBuilder.PlaceholderFormat(squirrel.Dollar)},
		pool: mock,
		pg:   pg,
	}
	ctx := context.Background()

	t.Run("should execute function in transaction successfully", func(t *testing.T) {
		mock.ExpectBegin()
		mock.ExpectCommit()

		executed := false
		err := pgRepo.InTransaction(ctx, func(repo order.TxOrderRepo) error {
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
		err := pgRepo.InTransaction(ctx, func(repo order.TxOrderRepo) error {
			return expectedErr
		})

		require.Error(t, err)
		assert.Equal(t, expectedErr, err)
	})

	t.Run("should handle begin transaction error", func(t *testing.T) {
		mock.ExpectBegin().WillReturnError(assert.AnError)

		err := pgRepo.InTransaction(ctx, func(repo order.TxOrderRepo) error {
			return nil
		})

		require.Error(t, err)
		assert.Contains(t, err.Error(), "begin transaction")
	})

	t.Run("should handle commit error", func(t *testing.T) {
		mock.ExpectBegin()
		mock.ExpectCommit().WillReturnError(assert.AnError)

		err := pgRepo.InTransaction(ctx, func(repo order.TxOrderRepo) error {
			return nil
		})

		require.Error(t, err)
		assert.Contains(t, err.Error(), "commit transaction")
	})

	t.Run("should handle rollback error after function error", func(t *testing.T) {
		mock.ExpectBegin()
		mock.ExpectRollback().WillReturnError(assert.AnError)

		functionErr := assert.AnError
		err := pgRepo.InTransaction(ctx, func(repo order.TxOrderRepo) error {
			return functionErr
		})

		require.Error(t, err)
		// Should return the original function error, not the rollback error
		assert.Equal(t, functionErr, err)
	})
}

func TestUpdateOrderHold(t *testing.T) {
	mock, err := pgxmock.NewPool()
	require.NoError(t, err)
	defer mock.Close()

	repo := &repo{db: mock, builder: squirrel.StatementBuilder.PlaceholderFormat(squirrel.Dollar)}
	ctx := context.Background()

	t.Run("should set order on hold successfully", func(t *testing.T) {
		reason := "manual_review"
		request := order.UpdateOrderHoldRequest{
			OrderID: "order-1",
			OnHold:  true,
			Reason:  &reason,
		}

		mock.ExpectExec(`UPDATE orders SET on_hold = \$1, hold_reason = \$2, updated_at = \$3 WHERE id = \$4`).
			WithArgs(true, &reason, "NOW()", "order-1").
			WillReturnResult(pgxmock.NewResult("UPDATE", 1))

		err := repo.UpdateOrderHold(ctx, request)

		require.NoError(t, err)
	})

	t.Run("should clear order hold successfully", func(t *testing.T) {
		request := order.UpdateOrderHoldRequest{
			OrderID: "order-1",
			OnHold:  false,
			Reason:  nil,
		}

		mock.ExpectExec(`UPDATE orders SET on_hold = \$1, hold_reason = \$2, updated_at = \$3 WHERE id = \$4`).
			WithArgs(false, (*string)(nil), "NOW()", "order-1").
			WillReturnResult(pgxmock.NewResult("UPDATE", 1))

		err := repo.UpdateOrderHold(ctx, request)

		require.NoError(t, err)
	})

	t.Run("should handle database error", func(t *testing.T) {
		request := order.UpdateOrderHoldRequest{
			OrderID: "order-1",
			OnHold:  true,
			Reason:  nil,
		}

		mock.ExpectExec(`UPDATE orders SET on_hold = \$1, hold_reason = \$2, updated_at = \$3 WHERE id = \$4`).
			WillReturnError(assert.AnError)

		err := repo.UpdateOrderHold(ctx, request)

		require.Error(t, err)
		assert.Contains(t, err.Error(), "update order hold")
	})
}
