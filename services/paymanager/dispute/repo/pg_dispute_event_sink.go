package repo

import (
	"TestTaskJustPay/pkg/postgres"
	"TestTaskJustPay/services/paymanager/dispute"
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"time"

	"github.com/Masterminds/squirrel"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
)

type PgDisputeEventRepo struct {
	db      postgres.Executor
	readDB  postgres.Executor
	builder squirrel.StatementBuilderType
}

var _ dispute.DisputeEvents = (*PgDisputeEventRepo)(nil)

func NewEventSink(db postgres.Executor, readDB postgres.Executor, builder squirrel.StatementBuilderType) *PgDisputeEventRepo {
	return &PgDisputeEventRepo{
		db:      db,
		readDB:  readDB,
		builder: builder,
	}
}

func (r *PgDisputeEventRepo) CreateDisputeEvent(ctx context.Context, event dispute.NewDisputeEvent) (*dispute.DisputeEvent, error) {
	id := uuid.New().String()

	query, args, err := r.builder.Insert("dispute_events").
		Columns("id", "dispute_id", "kind", "provider_event_id", "data", "created_at").
		Values(id, event.DisputeID, event.Kind, event.ProviderEventID, event.Data, event.CreatedAt).
		ToSql()
	if err != nil {
		return nil, fmt.Errorf("build insert query: %w", err)
	}

	_, err = r.db.Exec(ctx, query, args...)
	if postgres.IsPgErrorUniqueViolation(err) {
		return nil, dispute.ErrEventAlreadyStored
	}
	if err != nil {
		return nil, fmt.Errorf("create dispute event: %w", err)
	}

	return &dispute.DisputeEvent{
		EventID:         id,
		NewDisputeEvent: event,
	}, nil
}

func (r *PgDisputeEventRepo) GetDisputeEvents(ctx context.Context, query dispute.DisputeEventQuery) (dispute.DisputeEventPage, error) {
	if query.Limit <= 0 {
		query.Limit = 10
	}
	if query.Limit > 1000 {
		query.Limit = 1000
	}

	sqlQuery, args, err := r.buildDisputeEventPageQuery(query)
	if err != nil {
		return dispute.DisputeEventPage{}, fmt.Errorf("build dispute event query: %w", err)
	}

	rows, err := r.readDB.Query(ctx, sqlQuery, args...)
	if err != nil {
		return dispute.DisputeEventPage{}, fmt.Errorf("query dispute events: %w", err)
	}
	defer rows.Close()

	items, err := parseDisputeEventRows(rows)
	if err != nil {
		return dispute.DisputeEventPage{}, fmt.Errorf("parse dispute events: %w", err)
	}

	hasMore := len(items) > query.Limit
	if hasMore {
		items = items[:query.Limit]
	}

	var nextCursor string
	if hasMore && len(items) > 0 {
		lastItem := items[len(items)-1]
		nextCursor = encodeEventCursor(eventCursor{
			EventID:   lastItem.EventID,
			CreatedAt: lastItem.CreatedAt,
		})
	}

	return dispute.DisputeEventPage{
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

func (r *PgDisputeEventRepo) buildDisputeEventPageQuery(q dispute.DisputeEventQuery) (string, []any, error) {
	b := r.builder.Select("id", "dispute_id", "kind", "provider_event_id", "data", "created_at").
		From("dispute_events")

	if len(q.DisputeIDs) > 0 {
		b = b.Where(squirrel.Eq{"dispute_id": q.DisputeIDs})
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

func parseDisputeEventRows(rows pgx.Rows) ([]dispute.DisputeEvent, error) {
	var events []dispute.DisputeEvent
	for rows.Next() {
		var e dispute.DisputeEvent
		var rawKind string
		err := rows.Scan(&e.EventID, &e.DisputeID, &rawKind, &e.ProviderEventID, &e.Data, &e.CreatedAt)
		if err != nil {
			return nil, fmt.Errorf("scan dispute event row: %w", err)
		}

		e.Kind = dispute.DisputeEventKind(rawKind)
		events = append(events, e)
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate dispute event rows: %w", err)
	}

	return events, nil
}
