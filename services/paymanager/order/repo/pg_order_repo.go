package repo

import (
	"context"
	"fmt"
	"strings"

	"TestTaskJustPay/pkg/postgres"
	"TestTaskJustPay/services/paymanager/order"

	"github.com/Masterminds/squirrel"
	"github.com/jackc/pgx/v5"
)

type PgOrderRepo struct {
	pg *postgres.Postgres
	repo
}

func New(pg *postgres.Postgres, readDB postgres.Executor) order.OrderRepo {
	return &PgOrderRepo{
		pg:   pg,
		repo: repo{db: pg.Pool, readDB: readDB, builder: pg.Builder},
	}
}

func TxRepoFactory(builder squirrel.StatementBuilderType) func(postgres.Executor) order.OrderRepo {
	return func(tx postgres.Executor) order.OrderRepo {
		return &repo{db: tx, readDB: tx, builder: builder}
	}
}

type repo struct {
	db      postgres.Executor
	readDB  postgres.Executor
	builder squirrel.StatementBuilderType
}

func (r *repo) GetOrders(ctx context.Context, query *order.OrdersQuery) ([]order.Order, error) {
	sql, args := r.buildOrdersQuery(query)
	rows, err := r.readDB.Query(ctx, sql, args...)
	if err != nil {
		return nil, fmt.Errorf("query orders: %w", err)
	}
	defer rows.Close()

	return parseOrderRows(rows)
}

func (r *repo) UpdateOrder(ctx context.Context, update order.OrderUpdate) error {
	query, args, err := r.builder.Update("orders").
		Set("status", update.Status).
		Set("updated_at", update.UpdatedAt).
		Where(squirrel.Eq{"id": update.OrderId}).
		ToSql()
	if err != nil {
		return fmt.Errorf("build update query: %w", err)
	}

	_, err = r.db.Exec(ctx, query, args...)
	if err != nil {
		return fmt.Errorf("update order: %w", err)
	}
	return nil
}

func (r *repo) UpdateOrderHold(ctx context.Context, request order.UpdateOrderHoldRequest) error {
	query, args, err := r.builder.Update("orders").
		Set("on_hold", request.OnHold).
		Set("hold_reason", request.Reason).
		Set("updated_at", "NOW()").
		Where(squirrel.Eq{"id": request.OrderID}).
		ToSql()
	if err != nil {
		return fmt.Errorf("build update hold query: %w", err)
	}

	_, err = r.db.Exec(ctx, query, args...)
	if err != nil {
		return fmt.Errorf("update order hold: %w", err)
	}
	return nil
}

func (r *repo) CreateOrder(ctx context.Context, update order.OrderUpdate) error {
	query, args, err := r.builder.Insert("orders").
		Columns("id", "user_id", "status", "created_at", "updated_at").
		Values(update.OrderId, update.UserId, update.Status, update.CreatedAt, update.UpdatedAt).
		ToSql()
	if err != nil {
		return fmt.Errorf("build insert query: %w", err)
	}

	_, err = r.db.Exec(ctx, query, args...)
	if err != nil {
		if strings.Contains(err.Error(), "duplicate key") && strings.Contains(err.Error(), "orders_pkey") {
			return order.ErrAlreadyExists
		}
		return fmt.Errorf("create order by event: %w", err)
	}
	return nil
}

func (r *repo) buildOrdersQuery(q *order.OrdersQuery) (string, []any) {
	query := r.builder.Select("id", "user_id", "status", "on_hold", "hold_reason", "created_at", "updated_at").
		From("orders")

	if len(q.IDs) > 0 {
		query = query.Where(squirrel.Eq{"id": q.IDs})
	}

	if len(q.UserIDs) > 0 {
		query = query.Where(squirrel.Eq{"user_id": q.UserIDs})
	}

	if len(q.Statuses) > 0 {
		query = query.Where(squirrel.Eq{"status": q.Statuses})
	}

	if q.SortBy != nil && q.SortOrder != nil {
		query = query.OrderBy(fmt.Sprintf("%s %s", *q.SortBy, *q.SortOrder))
	}

	if q.Pagination != nil {
		offset := (q.Pagination.PageNumber - 1) * q.Pagination.PageSize
		query = query.Limit(uint64(q.Pagination.PageSize)).Offset(uint64(offset))
	}

	sql, args, _ := query.ToSql()
	return sql, args
}

func parseOrderRows(rows pgx.Rows) ([]order.Order, error) {
	var orders []order.Order
	for rows.Next() {
		var o order.Order
		var rawStatus string
		err := rows.Scan(&o.OrderId, &o.UserId, &rawStatus, &o.OnHold, &o.HoldReason, &o.CreatedAt, &o.UpdatedAt)
		if err != nil {
			return nil, fmt.Errorf("scan order row: %w", err)
		}

		status, err := order.NewStatus(rawStatus)
		if err != nil {
			return nil, fmt.Errorf("invalid status in database: %w", err)
		}
		o.Status = status

		orders = append(orders, o)
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate order rows: %w", err)
	}

	return orders, nil
}
