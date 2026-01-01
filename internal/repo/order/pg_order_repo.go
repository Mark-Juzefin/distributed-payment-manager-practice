package order_repo

import (
	"context"
	"fmt"
	"strings"

	"TestTaskJustPay/internal/domain/order"
	"TestTaskJustPay/pkg/postgres"

	"github.com/Masterminds/squirrel"
	"github.com/jackc/pgx/v5"
)

// PgOrderRepo is the main repository
type PgOrderRepo struct {
	pg *postgres.Postgres
	repo
}

func NewPgOrderRepo(pg *postgres.Postgres) order.OrderRepo {
	return &PgOrderRepo{
		pg:   pg,
		repo: repo{db: pg.Pool, builder: pg.Builder},
	}
}

func (r *PgOrderRepo) InTransaction(ctx context.Context, fn func(repo order.TxOrderRepo) error) error {
	return r.pg.InTransaction(ctx, func(tx postgres.Executor) error {
		txRepo := &repo{db: tx, builder: r.pg.Builder}
		return fn(txRepo)
	})
}

type repo struct {
	db      postgres.Executor
	builder squirrel.StatementBuilderType
}

func (r *repo) GetOrders(ctx context.Context, query *order.OrdersQuery) ([]order.Order, error) {
	sql, args := r.buildOrdersQuery(query)
	rows, err := r.db.Query(ctx, sql, args...)
	if err != nil {
		return nil, fmt.Errorf("query orders: %w", err)
	}
	defer rows.Close()

	return parseOrderRows(rows)
}

func (r *repo) UpdateOrder(ctx context.Context, event order.PaymentWebhook) error {
	query, args, err := r.builder.Update("orders").
		Set("status", event.Status).
		Set("updated_at", event.UpdatedAt).
		Where(squirrel.Eq{"id": event.OrderId}).
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

func (r *repo) CreateOrder(ctx context.Context, event order.PaymentWebhook) error {
	query, args, err := r.builder.Insert("orders").
		Columns("id", "user_id", "status", "created_at", "updated_at").
		Values(event.OrderId, event.UserId, event.Status, event.CreatedAt, event.UpdatedAt).
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

func (r *repo) buildOrdersQuery(q *order.OrdersQuery) (string, []interface{}) {
	query := r.builder.Select("id", "user_id", "status", "on_hold", "hold_reason", "created_at", "updated_at").
		From("orders")

	// Add WHERE conditions
	if len(q.IDs) > 0 {
		query = query.Where(squirrel.Eq{"id": q.IDs})
	}

	if len(q.UserIDs) > 0 {
		query = query.Where(squirrel.Eq{"user_id": q.UserIDs})
	}

	if len(q.Statuses) > 0 {
		query = query.Where(squirrel.Eq{"status": q.Statuses})
	}

	// Add sorting
	if q.SortBy != nil && q.SortOrder != nil {
		query = query.OrderBy(fmt.Sprintf("%s %s", *q.SortBy, *q.SortOrder))
	}

	// Add pagination
	if q.Pagination != nil {
		offset := (q.Pagination.PageNumber - 1) * q.Pagination.PageSize
		query = query.Limit(uint64(q.Pagination.PageSize)).Offset(uint64(offset))
	}

	sql, args, _ := query.ToSql()
	return sql, args
}

func (r *repo) buildEventsQuery(q *order.EventQuery) (string, []interface{}) {
	query := r.builder.Select("id", "order_id", "user_id", "status", "created_at", "updated_at").
		From("order_events").
		OrderBy("created_at DESC")

	// Add WHERE conditions
	if len(q.OrderIDs) > 0 {
		query = query.Where(squirrel.Eq{"order_id": q.OrderIDs})
	}

	if len(q.UserIDs) > 0 {
		query = query.Where(squirrel.Eq{"user_id": q.UserIDs})
	}

	if len(q.Statuses) > 0 {
		query = query.Where(squirrel.Eq{"status": q.Statuses})
	}

	sql, args, _ := query.ToSql()
	return sql, args
}

// Helper functions
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

func parseEventRows(rows pgx.Rows) ([]order.PaymentWebhook, error) {
	var events []order.PaymentWebhook
	for rows.Next() {
		var e order.PaymentWebhook
		var rawStatus string
		err := rows.Scan(&e.ProviderEventID, &e.OrderId, &e.UserId, &rawStatus, &e.CreatedAt, &e.UpdatedAt)
		if err != nil {
			return nil, fmt.Errorf("scan event row: %w", err)
		}

		status, err := order.NewStatus(rawStatus)
		if err != nil {
			return nil, fmt.Errorf("invalid status in database: %w", err)
		}
		e.Status = status

		events = append(events, e)
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate event rows: %w", err)
	}

	return events, nil
}
