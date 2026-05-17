package eventstore

import (
	"context"
	"fmt"

	"TestTaskJustPay/pkg/postgres"

	"github.com/Masterminds/squirrel"
	"github.com/google/uuid"
)

type PgEventStore struct {
	db      postgres.Executor
	builder squirrel.StatementBuilderType
}

var _ Store = (*PgEventStore)(nil)

func NewPgEventStore(db postgres.Executor, builder squirrel.StatementBuilderType) *PgEventStore {
	return &PgEventStore{
		db:      db,
		builder: builder,
	}
}

func TxStoreFactory(builder squirrel.StatementBuilderType) func(postgres.Executor) Store {
	return func(tx postgres.Executor) Store {
		return NewPgEventStore(tx, builder)
	}
}

func (s *PgEventStore) CreateEvent(ctx context.Context, event NewEvent) (*Event, error) {
	id := uuid.New().String()

	query, args, err := s.builder.Insert("events").
		Columns("id", "aggregate_type", "aggregate_id", "event_type", "idempotency_key", "payload", "created_at").
		Values(id, event.AggregateType, event.AggregateID, event.EventType, event.IdempotencyKey, event.Payload, event.CreatedAt).
		ToSql()
	if err != nil {
		return nil, fmt.Errorf("build insert query: %w", err)
	}

	_, err = s.db.Exec(ctx, query, args...)
	if postgres.IsPgErrorUniqueViolation(err) {
		return nil, ErrEventAlreadyStored
	}
	if err != nil {
		return nil, fmt.Errorf("create event: %w", err)
	}

	return &Event{
		ID:       id,
		NewEvent: event,
	}, nil
}
