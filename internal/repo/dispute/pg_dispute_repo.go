package dispute_repo

import (
	"TestTaskJustPay/internal/domain/dispute"
	"TestTaskJustPay/pkg/postgres"
	"context"
	"database/sql"
	"fmt"

	"github.com/Masterminds/squirrel"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
)

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
		Columns("id", "order_id", "status", "reason", "amount", "currency", "opened_at", "evidence_due_at", "submitted_at", "closed_at").
		Values(id, newDispute.OrderID, newDispute.Status, newDispute.Reason, newDispute.Amount, newDispute.Currency, newDispute.OpenedAt, newDispute.EvidenceDueAt, newDispute.SubmittedAt, newDispute.ClosedAt).
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

func (r *repo) buildDisputeByIDQuery(disputeID string) (string, []interface{}) {
	query := r.builder.Select("id", "order_id", "status", "reason", "amount", "currency", "opened_at", "evidence_due_at", "submitted_at", "closed_at").
		From("disputes").
		Where(squirrel.Eq{"id": disputeID})

	sql, args, err := query.ToSql()
	if err != nil {
		return "", nil
	}

	return sql, args
}

func (r *repo) buildDisputeByOrderIDQuery(orderID string) (string, []interface{}) {
	query := r.builder.Select("id", "order_id", "status", "reason", "amount", "currency", "opened_at", "evidence_due_at", "submitted_at", "closed_at").
		From("disputes").
		Where(squirrel.Eq{"order_id": orderID})

	sql, args, err := query.ToSql()
	if err != nil {
		return "", nil
	}

	return sql, args
}

// Helper functions
func parseDisputeRows(rows pgx.Rows) ([]dispute.Dispute, error) {
	var disputes []dispute.Dispute
	for rows.Next() {
		var d dispute.Dispute
		var rawStatus string
		var evidenceDueAt, submittedAt, closedAt sql.NullTime
		err := rows.Scan(&d.ID, &d.OrderID, &rawStatus, &d.Reason, &d.Amount, &d.Currency, &d.OpenedAt, &evidenceDueAt, &submittedAt, &closedAt)
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
