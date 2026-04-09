package inbox

//go:generate mockgen -source pg_inbox_repo.go -destination mock_inbox_repo.go -package inbox

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"TestTaskJustPay/pkg/postgres"

	"github.com/Masterminds/squirrel"
	"github.com/jackc/pgx/v5"
)

var ErrAlreadyExists = errors.New("inbox message already exists")

// NewInboxMessage represents a new webhook payload to store in the inbox.
type NewInboxMessage struct {
	IdempotencyKey string
	WebhookType    string
	Payload        json.RawMessage
}

// InboxMessage represents a full inbox row fetched for processing.
type InboxMessage struct {
	ID             string
	IdempotencyKey string
	WebhookType    string
	Payload        json.RawMessage
	RetryCount     int
	ReceivedAt     time.Time
}

// InboxRepo defines the interface for inbox persistence.
type InboxRepo interface {
	Store(ctx context.Context, msg NewInboxMessage) error
	FetchPending(ctx context.Context, limit int) ([]InboxMessage, error)
	MarkProcessed(ctx context.Context, id string) error
	MarkFailed(ctx context.Context, id string, errMsg string, maxRetries int) error
}

// PgInboxRepo implements InboxRepo using PostgreSQL.
type PgInboxRepo struct {
	db      postgres.Executor
	builder squirrel.StatementBuilderType
}

var _ InboxRepo = (*PgInboxRepo)(nil)

func NewPgInboxRepo(db postgres.Executor, builder squirrel.StatementBuilderType) *PgInboxRepo {
	return &PgInboxRepo{
		db:      db,
		builder: builder,
	}
}

func (r *PgInboxRepo) Store(ctx context.Context, msg NewInboxMessage) error {
	query, args, err := r.builder.Insert("inbox").
		Columns("idempotency_key", "webhook_type", "payload").
		Values(msg.IdempotencyKey, msg.WebhookType, msg.Payload).
		ToSql()
	if err != nil {
		return fmt.Errorf("build insert query: %w", err)
	}

	_, err = r.db.Exec(ctx, query, args...)
	if postgres.IsPgErrorUniqueViolation(err) {
		return ErrAlreadyExists
	}
	if err != nil {
		return fmt.Errorf("store inbox message: %w", err)
	}

	return nil
}

// FetchPending atomically claims up to `limit` pending messages by setting their status
// to 'processing'. Uses FOR UPDATE SKIP LOCKED to avoid contention between workers.
func (r *PgInboxRepo) FetchPending(ctx context.Context, limit int) ([]InboxMessage, error) {
	query := `
		UPDATE inbox SET status = 'processing'
		WHERE id IN (
			SELECT id FROM inbox
			WHERE status = 'pending'
			ORDER BY received_at ASC
			LIMIT $1
			FOR UPDATE SKIP LOCKED
		)
		RETURNING id, idempotency_key, webhook_type, payload, retry_count, received_at`

	rows, err := r.db.Query(ctx, query, limit)
	if err != nil {
		return nil, fmt.Errorf("fetch pending inbox messages: %w", err)
	}
	defer rows.Close()

	var messages []InboxMessage
	for rows.Next() {
		var msg InboxMessage
		if err := rows.Scan(
			&msg.ID,
			&msg.IdempotencyKey,
			&msg.WebhookType,
			&msg.Payload,
			&msg.RetryCount,
			&msg.ReceivedAt,
		); err != nil {
			return nil, fmt.Errorf("scan inbox message: %w", err)
		}
		messages = append(messages, msg)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate inbox rows: %w", err)
	}

	return messages, nil
}

// MarkProcessed sets a message status to 'processed' with a timestamp.
func (r *PgInboxRepo) MarkProcessed(ctx context.Context, id string) error {
	query := `UPDATE inbox SET status = 'processed', processed_at = NOW() WHERE id = $1`

	tag, err := r.db.Exec(ctx, query, id)
	if err != nil {
		return fmt.Errorf("mark inbox message processed: %w", err)
	}
	if tag.RowsAffected() == 0 {
		return pgx.ErrNoRows
	}

	return nil
}

// MarkFailed increments retry count and sets error message.
// If retry_count reaches maxRetries, status becomes 'failed' (permanent).
// Otherwise, status resets to 'pending' for re-pickup.
func (r *PgInboxRepo) MarkFailed(ctx context.Context, id string, errMsg string, maxRetries int) error {
	query := `
		UPDATE inbox
		SET retry_count = retry_count + 1,
		    error_message = $2,
		    status = CASE WHEN retry_count + 1 >= $3 THEN 'failed' ELSE 'pending' END
		WHERE id = $1`

	tag, err := r.db.Exec(ctx, query, id, errMsg, maxRetries)
	if err != nil {
		return fmt.Errorf("mark inbox message failed: %w", err)
	}
	if tag.RowsAffected() == 0 {
		return pgx.ErrNoRows
	}

	return nil
}
