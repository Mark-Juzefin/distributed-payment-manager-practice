package order_repo

import (
	"TestTaskJustPay/internal/api/domain/order"
	"context"
	"testing"
	"time"

	"github.com/Masterminds/squirrel"
	"github.com/google/uuid"
	"github.com/pashagolub/pgxmock/v4"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

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

		event := order.OrderUpdate{
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

		event := order.OrderUpdate{

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

		event := order.OrderUpdate{
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

		event := order.OrderUpdate{
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
