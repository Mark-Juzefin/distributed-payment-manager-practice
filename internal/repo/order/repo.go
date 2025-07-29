package order_repo

import (
	"TestTaskJustPay/internal/controller/apperror"
	"TestTaskJustPay/internal/domain/order"
	"TestTaskJustPay/pkg/postgres"
	"context"
	"errors"
	"fmt"

	"github.com/Masterminds/squirrel"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
)

// PgOrderRepo is the main repository
type PgOrderRepo struct {
	pg *postgres.Postgres
	repo
}

func NewPgOrderRepo(pg *postgres.Postgres) order.OrderRepo {
	return &PgOrderRepo{
		pg:   pg,
		repo: repo{pg.Pool, pg},
	}
}

func (r *PgOrderRepo) InTransaction(ctx context.Context, fn func(repo order.TxOrderRepo) error) error {
	return r.pg.InTransaction(ctx, func(tx postgres.Executor) error {
		txRepo := &repo{db: tx, pg: r.pg}
		return fn(txRepo)
	})
}

type repo struct {
	db postgres.Executor
	pg *postgres.Postgres
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

func (r *repo) GetEvents(ctx context.Context, query *order.EventQuery) ([]order.EventBase, error) {
	sql, args := r.buildEventsQuery(query)
	rows, err := r.db.Query(ctx, sql, args...)
	if err != nil {
		return nil, fmt.Errorf("query events: %w", err)
	}
	defer rows.Close()

	return parseEventRows(rows)
}

func (r *repo) UpdateOrder(ctx context.Context, event order.Event) error {
	query, args, err := r.pg.Builder.Update("order").
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

func (r *repo) CreateEvent(ctx context.Context, event order.Event) error {
	query, args, err := r.pg.Builder.Insert("event").
		Columns("id", "order_id", "user_id", "status", "created_at", "updated_at", "meta").
		Values(event.EventId, event.OrderId, event.UserId, event.Status, event.CreatedAt, event.UpdatedAt, event.Meta).
		ToSql()
	if err != nil {
		return fmt.Errorf("build insert query: %w", err)
	}

	_, err = r.db.Exec(ctx, query, args...)
	if isPgErrorUniqueViolation(err) {
		return apperror.ErrEventAlreadyStored
	}
	if err != nil {
		return fmt.Errorf("create event: %w", err)
	}
	return nil
}

func (r *repo) CreateOrderByEvent(ctx context.Context, event order.Event) error {
	query, args, err := r.pg.Builder.Insert("order").
		Columns("id", "user_id", "status", "created_at", "updated_at").
		Values(event.OrderId, event.UserId, event.Status, event.CreatedAt, event.UpdatedAt).
		ToSql()
	if err != nil {
		return fmt.Errorf("build insert query: %w", err)
	}

	_, err = r.db.Exec(ctx, query, args...)
	if err != nil {
		return fmt.Errorf("create order by event: %w", err)
	}
	return nil
}

func (r *repo) buildOrdersQuery(q *order.OrdersQuery) (string, []interface{}) {
	query := r.pg.Builder.Select("id", "user_id", "status", "created_at", "updated_at").
		From("order")

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

	sql, args, err := query.ToSql()
	if err != nil {
		return "", nil
	}

	return sql, args
}

func (r *repo) buildEventsQuery(q *order.EventQuery) (string, []interface{}) {
	query := r.pg.Builder.Select("id", "order_id", "user_id", "status", "created_at", "updated_at").
		From("event").
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

	sql, args, err := query.ToSql()
	if err != nil {
		return "", nil
	}

	return sql, args
}

// Helper functions
func parseOrderRows(rows pgx.Rows) ([]order.Order, error) {
	var orders []order.Order
	for rows.Next() {
		var o order.Order
		var rawStatus string
		err := rows.Scan(&o.OrderId, &o.UserId, &rawStatus, &o.CreatedAt, &o.UpdatedAt)
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

func parseEventRows(rows pgx.Rows) ([]order.EventBase, error) {
	var events []order.EventBase
	for rows.Next() {
		var e order.EventBase
		var rawStatus string
		err := rows.Scan(&e.EventId, &e.OrderId, &e.UserId, &rawStatus, &e.CreatedAt, &e.UpdatedAt)
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

func isPgErrorUniqueViolation(err error) bool {
	var pgErr *pgconn.PgError
	if errors.As(err, &pgErr) {
		return pgErr.Code == "23505"
	}
	return false
}
