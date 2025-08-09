package order_repo

import (
	"TestTaskJustPay/internal/controller/apperror"
	"TestTaskJustPay/internal/domain/order"
	"TestTaskJustPay/pkg/postgres"
	"context"
	"fmt"
	"testing"
	"time"

	"github.com/Masterminds/squirrel"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgconn"
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
		userId := uuid.New()
		expectedTime := time.Now()

		query := &order.OrdersQuery{
			IDs: []string{"order-1", "order-2"},
		}

		rows := mock.NewRows([]string{"id", "user_id", "status", "created_at", "updated_at"}).
			AddRow("order-1", userId, "created", expectedTime, expectedTime).
			AddRow("order-2", userId, "updated", expectedTime, expectedTime)

		mock.ExpectQuery(`SELECT id, user_id, status, created_at, updated_at FROM orders WHERE id IN \(\$1,\$2\)`).
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

func TestGetEvents(t *testing.T) {
	mock, err := pgxmock.NewPool()
	require.NoError(t, err)
	defer mock.Close()

	repo := &repo{db: mock, builder: squirrel.StatementBuilder.PlaceholderFormat(squirrel.Dollar)}
	ctx := context.Background()

	t.Run("should return events with filters", func(t *testing.T) {
		userId := uuid.New()
		expectedTime := time.Now()

		query := &order.EventQuery{
			OrderIDs: []string{"order-1"},
			UserIDs:  []string{userId.String()},
			Statuses: []order.Status{order.StatusCreated},
		}

		rows := mock.NewRows([]string{"id", "order_id", "user_id", "status", "created_at", "updated_at"}).
			AddRow("event-1", "order-1", userId, "created", expectedTime, expectedTime)

		mock.ExpectQuery(`SELECT id, order_id, user_id, status, created_at, updated_at FROM order_events WHERE order_id IN \(\$1\) AND user_id IN \(\$2\) AND status IN \(\$3\) ORDER BY created_at DESC`).
			WithArgs("order-1", userId.String(), order.StatusCreated).
			WillReturnRows(rows)

		result, err := repo.GetEvents(ctx, query)

		require.NoError(t, err)
		assert.Len(t, result, 1)
		assert.Equal(t, "event-1", result[0].EventId)
		assert.Equal(t, "order-1", result[0].OrderId)
		assert.Equal(t, order.StatusCreated, result[0].Status)
	})

	t.Run("should return events without filters", func(t *testing.T) {
		userId := uuid.New()
		expectedTime := time.Now()

		query := &order.EventQuery{}

		rows := mock.NewRows([]string{"id", "order_id", "user_id", "status", "created_at", "updated_at"}).
			AddRow("event-1", "order-1", userId, "created", expectedTime, expectedTime)

		mock.ExpectQuery(`SELECT id, order_id, user_id, status, created_at, updated_at FROM order_events ORDER BY created_at DESC`).
			WillReturnRows(rows)

		result, err := repo.GetEvents(ctx, query)

		require.NoError(t, err)
		assert.Len(t, result, 1)
	})

	t.Run("should handle database error", func(t *testing.T) {
		query := &order.EventQuery{}

		mock.ExpectQuery(`SELECT id, order_id, user_id, status, created_at, updated_at FROM order_events ORDER BY created_at DESC`).
			WillReturnError(assert.AnError)

		result, err := repo.GetEvents(ctx, query)

		require.Error(t, err)
		assert.Nil(t, result)
		assert.Contains(t, err.Error(), "query events")
	})
}

func TestUpdateOrder(t *testing.T) {
	mock, err := pgxmock.NewPool()
	require.NoError(t, err)
	defer mock.Close()

	repo := &repo{db: mock, builder: squirrel.StatementBuilder.PlaceholderFormat(squirrel.Dollar)}
	ctx := context.Background()

	t.Run("should update order successfully", func(t *testing.T) {
		userId := uuid.New()
		updatedAt := time.Now()

		event := order.Event{
			EventBase: order.EventBase{
				OrderId:   "order-1",
				UserId:    userId,
				Status:    order.StatusUpdated,
				UpdatedAt: updatedAt,
			},
		}

		mock.ExpectExec(`UPDATE orders SET status = \$1, updated_at = \$2 WHERE id = \$3`).
			WithArgs(order.StatusUpdated, updatedAt, "order-1").
			WillReturnResult(pgxmock.NewResult("UPDATE", 1))

		err := repo.UpdateOrder(ctx, event)

		require.NoError(t, err)
	})

	t.Run("should handle database error", func(t *testing.T) {
		userId := uuid.New()

		event := order.Event{
			EventBase: order.EventBase{
				OrderId: "order-1",
				UserId:  userId,
				Status:  order.StatusUpdated,
			},
		}

		mock.ExpectExec(`UPDATE order SET status = \$1, updated_at = \$2 WHERE id = \$3`).
			WillReturnError(assert.AnError)

		err := repo.UpdateOrder(ctx, event)

		require.Error(t, err)
		assert.Contains(t, err.Error(), "update order")
	})
}

func TestCreateEvent(t *testing.T) {
	mock, err := pgxmock.NewPool()
	require.NoError(t, err)
	defer mock.Close()

	repo := &repo{db: mock, builder: squirrel.StatementBuilder.PlaceholderFormat(squirrel.Dollar)}
	ctx := context.Background()

	t.Run("should create event successfully", func(t *testing.T) {
		userId := uuid.New()
		createdAt := time.Now()
		updatedAt := time.Now()

		event := order.Event{
			EventBase: order.EventBase{
				EventId:   "event-1",
				OrderId:   "order-1",
				UserId:    userId,
				Status:    order.StatusCreated,
				CreatedAt: createdAt,
				UpdatedAt: updatedAt,
			},
			Meta: map[string]string{"key": "value"},
		}

		mock.ExpectExec(`INSERT INTO order_events \(id,order_id,user_id,status,created_at,updated_at,meta\) VALUES \(\$1,\$2,\$3,\$4,\$5,\$6,\$7\)`).
			WithArgs("event-1", "order-1", userId, order.StatusCreated, createdAt, updatedAt, event.Meta).
			WillReturnResult(pgxmock.NewResult("INSERT", 1))

		err := repo.CreateEvent(ctx, event)

		require.NoError(t, err)
	})

	t.Run("should handle unique violation error", func(t *testing.T) {
		userId := uuid.New()
		createdAt := time.Now()
		updatedAt := time.Now()

		event := order.Event{
			EventBase: order.EventBase{
				EventId:   "event-1",
				OrderId:   "order-1",
				UserId:    userId,
				Status:    order.StatusCreated,
				CreatedAt: createdAt,
				UpdatedAt: updatedAt,
			},
			Meta: map[string]string{},
		}

		// Mock PostgreSQL unique violation error
		pgErr := &pgconn.PgError{
			Code: "23505", // unique_violation
		}

		mock.ExpectExec(`INSERT INTO order_events \(id,order_id,user_id,status,created_at,updated_at,meta\) VALUES \(\$1,\$2,\$3,\$4,\$5,\$6,\$7\)`).
			WithArgs("event-1", "order-1", userId, order.StatusCreated, createdAt, updatedAt, event.Meta).
			WillReturnError(pgErr)

		err := repo.CreateEvent(ctx, event)

		require.Error(t, err)
		assert.Equal(t, apperror.ErrEventAlreadyStored, err)
	})

	t.Run("should handle other database errors", func(t *testing.T) {
		userId := uuid.New()

		event := order.Event{
			EventBase: order.EventBase{
				EventId: "event-1",
				OrderId: "order-1",
				UserId:  userId,
				Status:  order.StatusCreated,
			},
		}

		mock.ExpectExec(`INSERT INTO order_events \(id,order_id,user_id,status,created_at,updated_at,meta\) VALUES \(\$1,\$2,\$3,\$4,\$5,\$6,\$7\)`).
			WillReturnError(assert.AnError)

		err := repo.CreateEvent(ctx, event)

		require.Error(t, err)
		assert.Contains(t, err.Error(), "create event")
		assert.NotEqual(t, apperror.ErrEventAlreadyStored, err)
	})
}

func TestCreateOrderByEvent(t *testing.T) {
	mock, err := pgxmock.NewPool()
	require.NoError(t, err)
	defer mock.Close()

	repo := &repo{db: mock, builder: squirrel.StatementBuilder.PlaceholderFormat(squirrel.Dollar)}
	ctx := context.Background()

	t.Run("should create order successfully", func(t *testing.T) {
		userId := uuid.New()
		createdAt := time.Now()
		updatedAt := time.Now()

		event := order.Event{
			EventBase: order.EventBase{
				OrderId:   "order-1",
				UserId:    userId,
				Status:    order.StatusCreated,
				CreatedAt: createdAt,
				UpdatedAt: updatedAt,
			},
		}

		mock.ExpectExec(`INSERT INTO orders \(id,user_id,status,created_at,updated_at\) VALUES \(\$1,\$2,\$3,\$4,\$5\)`).
			WithArgs("order-1", userId, order.StatusCreated, createdAt, updatedAt).
			WillReturnResult(pgxmock.NewResult("INSERT", 1))

		err := repo.CreateOrderByEvent(ctx, event)

		require.NoError(t, err)
	})

	t.Run("should handle database error", func(t *testing.T) {
		userId := uuid.New()

		event := order.Event{
			EventBase: order.EventBase{
				OrderId: "order-1",
				UserId:  userId,
				Status:  order.StatusCreated,
			},
		}

		mock.ExpectExec(`INSERT INTO orders \(id,user_id,status,created_at,updated_at\) VALUES \(\$1,\$2,\$3,\$4,\$5\)`).
			WillReturnError(assert.AnError)

		err := repo.CreateOrderByEvent(ctx, event)

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
