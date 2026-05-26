package purchase

import (
	"context"

	"TestTaskJustPay/pkg/postgres"
	"TestTaskJustPay/services/silvergate/internal/product"
	"TestTaskJustPay/services/silvergate/internal/transaction"

	"github.com/google/uuid"
)

// Authorizer composes the acquirer call and the transaction insert using the
// caller-supplied repo so /purchase can run authorization atomically inside its
// own DB transaction.
type Authorizer interface {
	AuthorizeInTx(ctx context.Context, repo transaction.Repo, req transaction.AuthRequest) (*transaction.Transaction, error)
}

// Capturer kicks off the bank settlement leg of /purchase. Runs in its own DB
// transaction outside the purchase tx — see spec §Why Capture викликаємо ПОЗА.
type Capturer interface {
	Capture(ctx context.Context, req transaction.CaptureRequest) (transaction.CaptureResponse, error)
}

// ProductService is the subset of *product.Service that purchase composition needs.
type ProductService interface {
	Get(ctx context.Context, merchantID string, id uuid.UUID) (*product.Product, error)
	MarkPurchasedInTx(ctx context.Context, exec postgres.Executor, merchantID string, id uuid.UUID) error
}

// TxLookup is the pre-check side of idempotency — finds an existing transaction
// for (merchant_id, purchase_idempotency_key). Returns transaction.ErrNotFound
// when no row exists.
type TxLookup interface {
	GetByPurchaseIdempotencyKey(ctx context.Context, merchantID, key string) (*transaction.Transaction, error)
}
