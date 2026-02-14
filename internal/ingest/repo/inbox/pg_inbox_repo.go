package inbox

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"

	"TestTaskJustPay/pkg/postgres"

	"github.com/Masterminds/squirrel"
)

var ErrAlreadyExists = errors.New("inbox message already exists")

// NewInboxMessage represents a new webhook payload to store in the inbox.
type NewInboxMessage struct {
	IdempotencyKey string
	WebhookType    string
	Payload        json.RawMessage
}

// InboxRepo defines the interface for inbox persistence.
type InboxRepo interface {
	Store(ctx context.Context, msg NewInboxMessage) error
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
