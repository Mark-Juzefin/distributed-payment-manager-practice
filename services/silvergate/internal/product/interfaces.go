package product

import (
	"context"
	"time"

	"github.com/google/uuid"
)

// ListFilter — StatusFilter == nil → no status filter.
type ListFilter struct {
	StatusFilter *Status
	Cursor       *Cursor
	Limit        int
}

// Cursor encodes keyset pagination position over (created_at, id).
type Cursor struct {
	CreatedAt time.Time
	ID        uuid.UUID
}

// Repo: all mutations scope by merchantID — foreign products return ErrNotFound.
type Repo interface {
	Create(ctx context.Context, p *Product) error
	GetByID(ctx context.Context, merchantID string, id uuid.UUID) (*Product, error)
	List(ctx context.Context, merchantID string, filter ListFilter) ([]*Product, *Cursor, error)
	Update(ctx context.Context, merchantID string, id uuid.UUID, upd Update) error
	SetStatus(ctx context.Context, merchantID string, id uuid.UUID, status Status) error

	// MarkPurchased sets first_purchased_at = now() iff currently NULL. Idempotent.
	// merchantID required as defense-in-depth.
	MarkPurchased(ctx context.Context, merchantID string, id uuid.UUID) error
}
