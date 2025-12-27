package order_eventsink

import (
	"TestTaskJustPay/internal/controller/apperror"
	"TestTaskJustPay/internal/domain/order"
	"TestTaskJustPay/pkg/postgres"
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"time"

	"github.com/Masterminds/squirrel"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
)

type PgOrderEventRepo struct {
	db      postgres.Executor
	builder squirrel.StatementBuilderType
}

var _ order.EventSink = (*PgOrderEventRepo)(nil)

func NewPgOrderEventRepo(db postgres.Executor, builder squirrel.StatementBuilderType) *PgOrderEventRepo {
	return &PgOrderEventRepo{
		db:      db,
		builder: builder,
	}
}

func (r *PgOrderEventRepo) CreateOrderEvent(ctx context.Context, event order.NewOrderEvent) (*order.OrderEvent, error) {
	id := uuid.New().String()

	query, args, err := r.builder.Insert("order_events").
		Columns("id", "order_id", "kind", "provider_event_id", "data", "created_at").
		Values(id, event.OrderID, event.Kind, event.ProviderEventID, event.Data, event.CreatedAt).
		ToSql()
	if err != nil {
		return nil, fmt.Errorf("build insert query: %w", err)
	}

	_, err = r.db.Exec(ctx, query, args...)
	if postgres.IsPgErrorUniqueViolation(err) {
		return nil, apperror.ErrEventAlreadyStored
	}
	if err != nil {
		return nil, fmt.Errorf("create order event: %w", err)
	}

	return &order.OrderEvent{
		EventID:       id,
		NewOrderEvent: event,
	}, nil
}

func (r *PgOrderEventRepo) GetOrderEventByID(ctx context.Context, eventID string) (*order.OrderEvent, error) {
	query, args, err := r.builder.Select("id", "order_id", "kind", "provider_event_id", "data", "created_at").
		From("order_events").
		Where(squirrel.Eq{"id": eventID}).
		ToSql()
	if err != nil {
		return nil, fmt.Errorf("build get order event by id query: %w", err)
	}

	rows, err := r.db.Query(ctx, query, args...)
	if err != nil {
		return nil, fmt.Errorf("query order event by id: %w", err)
	}
	defer rows.Close()

	events, err := parseOrderEventRows(rows)
	if err != nil {
		return nil, fmt.Errorf("parse order event: %w", err)
	}

	if len(events) == 0 {
		return nil, fmt.Errorf("order event not found")
	}

	return &events[0], nil
}

func (r *PgOrderEventRepo) GetOrderEvents(ctx context.Context, query order.OrderEventQuery) (order.OrderEventPage, error) {
	if query.Limit <= 0 {
		query.Limit = 10
	}
	if query.Limit > 1000 {
		query.Limit = 1000
	}

	sqlQuery, args, err := r.buildOrderEventPageQuery(query)
	if err != nil {
		return order.OrderEventPage{}, fmt.Errorf("build order event query: %w", err)
	}

	rows, err := r.db.Query(ctx, sqlQuery, args...)
	if err != nil {
		return order.OrderEventPage{}, fmt.Errorf("query order events: %w", err)
	}
	defer rows.Close()

	items, err := parseOrderEventRows(rows)
	if err != nil {
		return order.OrderEventPage{}, fmt.Errorf("parse order events: %w", err)
	}

	hasMore := len(items) > query.Limit
	if hasMore {
		items = items[:query.Limit] // trim the extra item queried to determine the existence of the following items
	}

	var nextCursor string
	if hasMore && len(items) > 0 {
		lastItem := items[len(items)-1]
		nextCursor = encodeEventCursor(eventCursor{
			EventID:   lastItem.EventID,
			CreatedAt: lastItem.CreatedAt,
		})
	}

	return order.OrderEventPage{
		Items:      items,
		NextCursor: nextCursor,
		HasMore:    hasMore,
	}, nil
}

type eventCursor struct {
	EventID   string    `json:"event_id"`
	CreatedAt time.Time `json:"created_at"`
}

func encodeEventCursor(c eventCursor) string {
	b, _ := json.Marshal(c)
	return base64.StdEncoding.EncodeToString(b)
}
func decodeEventCursor(s string) (eventCursor, error) {
	var c eventCursor
	b, err := base64.StdEncoding.DecodeString(s)
	if err != nil {
		return c, err
	}
	return c, json.Unmarshal(b, &c)
}

// SELECT id, order_id, kind, created_at FROM order_events
// WHERE
//
//	order_id IN @OrderIDs
//	AND kind IN @Kinds
//	AND created_at >= @TimeFrom
//	AND created_at < @TimeTo
//	AND (created_at, id) < (@cursor.CreatedAt, @cursor.EventID)
//
// ORDER BY created_at DESC/ASC, id DESC/ASC
// LIMIT @Limit+1
func (r *PgOrderEventRepo) buildOrderEventPageQuery(q order.OrderEventQuery) (string, []interface{}, error) {
	b := r.builder.Select("id", "order_id", "kind", "provider_event_id", "data", "created_at").
		From("order_events")

	if len(q.OrderIDs) > 0 {
		b = b.Where(squirrel.Eq{"order_id": q.OrderIDs})
	}

	if len(q.Kinds) > 0 {
		b = b.Where(squirrel.Eq{"kind": q.Kinds})
	}

	if q.TimeFrom != nil {
		b = b.Where("created_at >= ?", q.TimeFrom.UTC())
	}

	if q.TimeTo != nil {
		b = b.Where("created_at < ?", q.TimeTo.UTC())
	}

	if q.Cursor != "" {
		cursor, err := decodeEventCursor(q.Cursor)
		if err != nil {
			return "", nil, fmt.Errorf("decode cursor: %w", err)
		}

		if q.SortAsc {
			b = b.Where("(created_at, id) > (?, ?)", cursor.CreatedAt.UTC(), cursor.EventID)
		} else {
			b = b.Where("(created_at, id) < (?, ?)", cursor.CreatedAt.UTC(), cursor.EventID)
		}
	}

	if q.SortAsc {
		b = b.OrderBy("created_at ASC", "id ASC")
	} else {
		b = b.OrderBy("created_at DESC", "id DESC")
	}

	b = b.Limit(uint64(q.Limit + 1))

	sql, args, _ := b.ToSql()
	return sql, args, nil
}

func parseOrderEventRows(rows pgx.Rows) ([]order.OrderEvent, error) {
	var events []order.OrderEvent
	for rows.Next() {
		var e order.OrderEvent
		var rawKind string
		err := rows.Scan(&e.EventID, &e.OrderID, &rawKind, &e.ProviderEventID, &e.Data, &e.CreatedAt)
		if err != nil {
			return nil, fmt.Errorf("scan order event row: %w", err)
		}

		e.Kind = order.OrderEventKind(rawKind)
		events = append(events, e)
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate order event rows: %w", err)
	}

	return events, nil
}
