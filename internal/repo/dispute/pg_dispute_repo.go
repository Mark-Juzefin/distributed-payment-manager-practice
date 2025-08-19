package dispute_repo

import (
	"TestTaskJustPay/internal/domain/dispute"
	"TestTaskJustPay/internal/domain/gateway"
	"TestTaskJustPay/pkg/postgres"
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"time"

	"github.com/Masterminds/squirrel"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
)

var disputeColumns = []string{"id", "order_id", "submitting_id", "status", "reason", "amount", "currency", "opened_at", "evidence_due_at", "submitted_at", "closed_at"}

type PgDisputeRepo struct {
	pg *postgres.Postgres
	repo
}

func NewPgDisputeRepo(pg *postgres.Postgres) dispute.DisputeRepo {
	return &PgDisputeRepo{
		pg:   pg,
		repo: repo{db: pg.Pool, builder: pg.Builder},
	}
}

func (r *PgDisputeRepo) InTransaction(ctx context.Context, fn func(repo dispute.TxDisputeRepo) error) error {
	return r.pg.InTransaction(ctx, func(tx postgres.Executor) error {
		txRepo := &repo{db: tx, builder: r.pg.Builder}
		return fn(txRepo)
	})
}

type repo struct {
	db      postgres.Executor
	builder squirrel.StatementBuilderType
}

func (r *repo) GetDisputes(ctx context.Context) ([]dispute.Dispute, error) {
	query, args := r.buildDisputesQuery()
	rows, err := r.db.Query(ctx, query, args...)
	if err != nil {
		return nil, fmt.Errorf("query disputes: %w", err)
	}
	defer rows.Close()

	return parseDisputeRows(rows)
}

func (r *repo) GetDisputeByID(ctx context.Context, disputeID string) (*dispute.Dispute, error) {
	query, args := r.buildDisputeByIDQuery(disputeID)
	rows, err := r.db.Query(ctx, query, args...)
	if err != nil {
		return nil, fmt.Errorf("query dispute by id: %w", err)
	}
	defer rows.Close()

	disputes, err := parseDisputeRows(rows)
	if err != nil {
		return nil, err
	}
	if len(disputes) == 0 {
		return nil, nil
	}
	return &disputes[0], nil
}
func (r *repo) GetDisputeByOrderID(ctx context.Context, orderID string) (*dispute.Dispute, error) {
	query, args := r.buildDisputeByOrderIDQuery(orderID)
	rows, err := r.db.Query(ctx, query, args...)
	if err != nil {
		return nil, fmt.Errorf("query dispute by order id: %w", err)
	}
	defer rows.Close()

	disputes, err := parseDisputeRows(rows)
	if err != nil {
		return nil, err
	}
	if len(disputes) == 0 {
		return nil, nil
	}
	return &disputes[0], nil
}

func (r *repo) CreateDispute(ctx context.Context, newDispute dispute.NewDispute) (*dispute.Dispute, error) {
	id := uuid.New().String()

	query, args, err := r.builder.Insert("disputes").
		Columns(disputeColumns...).
		Values(id, newDispute.OrderID, newDispute.SubmittingId, newDispute.Status, newDispute.Reason, newDispute.Amount, newDispute.Currency, newDispute.OpenedAt, newDispute.EvidenceDueAt, newDispute.SubmittedAt, newDispute.ClosedAt).
		ToSql()
	if err != nil {
		return nil, fmt.Errorf("build insert query: %w", err)
	}

	_, err = r.db.Exec(ctx, query, args...)
	if err != nil {
		return nil, fmt.Errorf("create dispute: %w", err)
	}

	return &dispute.Dispute{
		ID:          id,
		Status:      newDispute.Status,
		DisputeInfo: newDispute.DisputeInfo,
	}, nil
}
func (r *repo) UpdateDispute(ctx context.Context, disputeToUpdate dispute.Dispute) error {
	query, args, err := r.builder.Update("disputes").
		Set("submitting_id", disputeToUpdate.SubmittingId).
		Set("status", disputeToUpdate.Status).
		Set("reason", disputeToUpdate.Reason).
		Set("amount", disputeToUpdate.Amount).
		Set("currency", disputeToUpdate.Currency).
		Set("opened_at", disputeToUpdate.OpenedAt).
		Set("evidence_due_at", disputeToUpdate.EvidenceDueAt).
		Set("submitted_at", disputeToUpdate.SubmittedAt).
		Set("closed_at", disputeToUpdate.ClosedAt).
		Where(squirrel.Eq{"id": disputeToUpdate.ID}).
		ToSql()
	if err != nil {
		return fmt.Errorf("build update query: %w", err)
	}

	_, err = r.db.Exec(ctx, query, args...)
	if err != nil {
		return fmt.Errorf("update dispute: %w", err)
	}
	return nil
}

func (r *repo) CreateDisputeEvent(ctx context.Context, event dispute.NewDisputeEvent) error {
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

func (r *repo) GetDisputeEvents(ctx context.Context, query *dispute.DisputeEventQuery) ([]dispute.DisputeEvent, error) {
	sqlQuery, args := r.buildDisputeEventsQuery(query)

	rows, err := r.db.Query(ctx, sqlQuery, args...)
	if err != nil {
		return nil, fmt.Errorf("query dispute events: %w", err)
	}
	defer rows.Close()

	return parseDisputeEventRows(rows)
}

func (r *repo) UpsertEvidence(ctx context.Context, disputeID string, upsert dispute.EvidenceUpsert) (*dispute.Evidence, error) {
	query, args, err := r.buildUpsertEvidenceQuery(disputeID, upsert)
	if err != nil {
		return nil, fmt.Errorf("build upsert query: %w", err)
	}

	_, err = r.db.Exec(ctx, query, args...)
	if err != nil {
		return nil, fmt.Errorf("upsert evidence: %w", err)
	}

	return &dispute.Evidence{
		DisputeID: disputeID,
		Evidence: gateway.Evidence{
			Fields: upsert.Fields,
			Files:  upsert.Files,
		},
		UpdatedAt: time.Now(),
	}, nil
}

func (r *repo) GetEvidence(ctx context.Context, disputeID string) (*dispute.Evidence, error) {
	query, args := r.buildGetEvidenceQuery(disputeID)
	rows, err := r.db.Query(ctx, query, args...)
	if err != nil {
		return nil, fmt.Errorf("query evidence: %w", err)
	}
	defer rows.Close()

	evidence, err := parseEvidenceRows(rows)
	if err != nil {
		return nil, err
	}
	if len(evidence) == 0 {
		return nil, nil
	}
	return &evidence[0], nil
}

func (r *repo) buildUpsertEvidenceQuery(disputeID string, upsert dispute.EvidenceUpsert) (string, []interface{}, error) {
	fieldsJSON, err := json.Marshal(upsert.Fields)
	if err != nil {
		return "", nil, fmt.Errorf("marshal fields: %w", err)
	}

	filesJSON, err := json.Marshal(upsert.Files)
	if err != nil {
		return "", nil, fmt.Errorf("marshal files: %w", err)
	}

	now := time.Now()
	query := r.builder.Insert("evidence").
		Columns("dispute_id", "fields", "files", "updated_at").
		Values(disputeID, fieldsJSON, filesJSON, now).
		Suffix("ON CONFLICT (dispute_id) DO UPDATE SET fields = EXCLUDED.fields, files = EXCLUDED.files, updated_at = EXCLUDED.updated_at")

	return query.ToSql()
}

func (r *repo) buildGetEvidenceQuery(disputeID string) (string, []interface{}) {
	query := r.builder.Select("dispute_id", "fields", "files", "updated_at").
		From("evidence").
		Where(squirrel.Eq{"dispute_id": disputeID})

	sql, args, _ := query.ToSql()
	return sql, args
}

func (r *repo) buildDisputesQuery() (string, []interface{}) {
	query := r.builder.Select(disputeColumns...).
		From("disputes").
		OrderBy("opened_at DESC")

	sql, args, _ := query.ToSql()
	return sql, args
}

func (r *repo) buildDisputeByIDQuery(disputeID string) (string, []interface{}) {
	query := r.builder.Select(disputeColumns...).
		From("disputes").
		Where(squirrel.Eq{"id": disputeID})

	sql, args, _ := query.ToSql()
	return sql, args
}

func (r *repo) buildDisputeByOrderIDQuery(orderID string) (string, []interface{}) {
	query := r.builder.Select(disputeColumns...).
		From("disputes").
		Where(squirrel.Eq{"order_id": orderID})

	sql, args, _ := query.ToSql()
	return sql, args
}

func (r *repo) buildDisputeEventsQuery(eventQuery *dispute.DisputeEventQuery) (string, []interface{}) {
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

// Helper functions
func parseDisputeRows(rows pgx.Rows) ([]dispute.Dispute, error) {
	var disputes []dispute.Dispute
	for rows.Next() {
		var d dispute.Dispute
		var rawStatus string
		var evidenceDueAt, submittedAt, closedAt sql.NullTime
		err := rows.Scan(&d.ID, &d.OrderID, &d.SubmittingId, &rawStatus, &d.Reason, &d.Amount, &d.Currency, &d.OpenedAt, &evidenceDueAt, &submittedAt, &closedAt)
		if err != nil {
			return nil, fmt.Errorf("scan dispute row: %w", err)
		}

		d.Status = dispute.DisputeStatus(rawStatus)

		if evidenceDueAt.Valid {
			d.EvidenceDueAt = &evidenceDueAt.Time
		}
		if submittedAt.Valid {
			d.SubmittedAt = &submittedAt.Time
		}
		if closedAt.Valid {
			d.ClosedAt = &closedAt.Time
		}

		disputes = append(disputes, d)
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate dispute rows: %w", err)
	}

	return disputes, nil
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

func parseEvidenceRows(rows pgx.Rows) ([]dispute.Evidence, error) {
	var evidenceList []dispute.Evidence
	for rows.Next() {
		var e dispute.Evidence
		var fieldsJSON, filesJSON string
		err := rows.Scan(&e.DisputeID, &fieldsJSON, &filesJSON, &e.UpdatedAt)
		if err != nil {
			return nil, fmt.Errorf("scan evidence row: %w", err)
		}

		var fields map[string]string
		var files []gateway.EvidenceFile

		if err := json.Unmarshal([]byte(fieldsJSON), &fields); err != nil {
			return nil, fmt.Errorf("unmarshal fields: %w", err)
		}

		if err := json.Unmarshal([]byte(filesJSON), &files); err != nil {
			return nil, fmt.Errorf("unmarshal files: %w", err)
		}

		e.Evidence = gateway.Evidence{
			Fields: fields,
			Files:  files,
		}

		evidenceList = append(evidenceList, e)
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate evidence rows: %w", err)
	}

	return evidenceList, nil
}
