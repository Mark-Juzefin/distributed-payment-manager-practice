package eventsink

import (
	"TestTaskJustPay/internal/domain/dispute"
	"TestTaskJustPay/pkg/postgres"
	"context"
	"fmt"

	"github.com/Masterminds/squirrel"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
)

type PgEventRepo struct {
	db      postgres.Executor
	builder squirrel.StatementBuilderType
}

var _ dispute.EventSink = (*PgEventRepo)(nil)

func NewPgEventRepo(pg *postgres.Postgres) *PgEventRepo {
	return &PgEventRepo{
		db:      pg.Pool,
		builder: pg.Builder,
	}
}

func (r *PgEventRepo) CreateDisputeEvent(ctx context.Context, event dispute.NewDisputeEvent) error {
	id := uuid.New().String()

	query, args, err := r.builder.Insert("dispute_events").
		Columns("id", "dispute_id", "kind", "provider_event_id", "data", "created_at").
		Values(id, event.DisputeID, event.Kind, event.ProviderEventID, event.Data, event.CreatedAt).
		ToSql()
	if err != nil {
		return fmt.Errorf("build insert query: %w", err)
	}

	_, err = r.db.Exec(ctx, query, args...)
	if postgres.IsPgErrorUniqueViolation(err) {
		return fmt.Errorf("dispute event already exists: %w", err)
	}
	if err != nil {
		return fmt.Errorf("create dispute event: %w", err)
	}
	return nil
}

func (r *PgEventRepo) GetDisputeEvents(ctx context.Context, query *dispute.DisputeEventQuery) ([]dispute.DisputeEvent, error) {
	sqlQuery, args := r.buildDisputeEventsQuery(query)

	rows, err := r.db.Query(ctx, sqlQuery, args...)
	if err != nil {
		return nil, fmt.Errorf("query dispute events: %w", err)
	}
	defer rows.Close()

	return parseDisputeEventRows(rows)
}

func (r *PgEventRepo) buildDisputeEventsQuery(eventQuery *dispute.DisputeEventQuery) (string, []interface{}) {
	query := r.builder.Select("id", "dispute_id", "kind", "provider_event_id", "data", "created_at").
		From("dispute_events").
		OrderBy("created_at DESC")

	if len(eventQuery.DisputeIDs) > 0 {
		query = query.Where(squirrel.Eq{"dispute_id": eventQuery.DisputeIDs})
	}

	if len(eventQuery.Kinds) > 0 {
		query = query.Where(squirrel.Eq{"kind": eventQuery.Kinds})
	}

	sql, args, _ := query.ToSql()
	return sql, args
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
