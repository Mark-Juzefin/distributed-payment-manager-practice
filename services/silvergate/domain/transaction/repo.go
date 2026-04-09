package transaction

import (
	"context"

	"github.com/google/uuid"
)

type Repo interface {
	Create(ctx context.Context, tx *Transaction) error
	GetByID(ctx context.Context, id uuid.UUID) (*Transaction, error)
	UpdateStatus(ctx context.Context, tx *Transaction) error
}
