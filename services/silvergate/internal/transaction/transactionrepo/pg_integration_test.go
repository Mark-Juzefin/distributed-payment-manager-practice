//go:build integration

package transactionrepo_test

import (
	"context"
	"errors"
	"strings"
	"testing"

	"TestTaskJustPay/services/silvergate/internal/product"
	"TestTaskJustPay/services/silvergate/internal/product/productrepo"
	"TestTaskJustPay/services/silvergate/internal/transaction"
	"TestTaskJustPay/services/silvergate/internal/transaction/transactionrepo"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func merchantID(t *testing.T) string {
	t.Helper()
	return "m_" + strings.ReplaceAll(uuid.NewString(), "-", "")
}

func seedProduct(t *testing.T, ctx context.Context, merchant string) *product.Product {
	t.Helper()
	repo := productrepo.NewPgProductRepo(pg.Pool)
	p := product.New(merchant, "Widget", "Basic", 1999, "USD", nil)
	require.NoError(t, repo.Create(ctx, p))
	return p
}

func newPurchaseTx(merchant, orderRef, cardToken string, productID uuid.UUID, key string) *transaction.Transaction {
	tx := transaction.NewAuthorized(merchant, orderRef, 1999, "USD", cardToken)
	tx.MarkProductPurchase(key, productID)
	return tx
}

func TestCreate_PurchaseIdempotencyConstraint(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	repo := transactionrepo.NewPgTransactionRepo(pg.Pool)

	t.Run("duplicate (merchant, purchase_key) rejected with ErrPurchaseIdempotencyConflict", func(t *testing.T) {
		merchant := merchantID(t)
		p := seedProduct(t, ctx, merchant)
		first := newPurchaseTx(merchant, "ord1", "tok1", p.ID, "K1")
		require.NoError(t, repo.Create(ctx, first))

		dup := newPurchaseTx(merchant, "ord1", "tok1", p.ID, "K1")
		err := repo.Create(ctx, dup)
		assert.ErrorIs(t, err, transaction.ErrPurchaseIdempotencyConflict)
	})

	t.Run("NULL purchase_key allows multiple inserts (partial index)", func(t *testing.T) {
		merchant := merchantID(t)
		p := seedProduct(t, ctx, merchant)
		// /auth path leaves purchase context empty.
		tx1 := transaction.NewAuthorized(merchant, "o1", 100, "USD", "tok")
		tx2 := transaction.NewAuthorized(merchant, "o2", 200, "USD", "tok")
		require.NoError(t, repo.Create(ctx, tx1))
		require.NoError(t, repo.Create(ctx, tx2))
		// Reference product so the FK column path is exercised separately above.
		_ = p
	})

	t.Run("same key across different merchants allowed", func(t *testing.T) {
		merchantA := merchantID(t)
		merchantB := merchantID(t)
		pA := seedProduct(t, ctx, merchantA)
		pB := seedProduct(t, ctx, merchantB)
		txA := newPurchaseTx(merchantA, "ord", "tok", pA.ID, "SHARED_KEY")
		txB := newPurchaseTx(merchantB, "ord", "tok", pB.ID, "SHARED_KEY")
		require.NoError(t, repo.Create(ctx, txA))
		require.NoError(t, repo.Create(ctx, txB))
	})

	t.Run("capture overwrite on idempotency_key does not touch purchase_idempotency_key", func(t *testing.T) {
		merchant := merchantID(t)
		p := seedProduct(t, ctx, merchant)
		tx := newPurchaseTx(merchant, "ord", "tok", p.ID, "PURCHASE_K")
		require.NoError(t, repo.Create(ctx, tx))

		// Simulate /capture overwrite path: MarkCapturePending mutates IdempotencyKey,
		// UpdateStatus writes idempotency_key column.
		require.NoError(t, tx.MarkCapturePending("CAPTURE_K"))
		require.NoError(t, repo.UpdateStatus(ctx, tx))

		got, err := repo.GetByID(ctx, tx.ID)
		require.NoError(t, err)
		assert.Equal(t, "CAPTURE_K", got.IdempotencyKey, "capture key should overwrite idempotency_key column")
		assert.Equal(t, "PURCHASE_K", got.PurchaseIdempotencyKey, "purchase key must survive capture overwrite")
	})
}

func TestGetByPurchaseIdempotencyKey(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	repo := transactionrepo.NewPgTransactionRepo(pg.Pool)

	t.Run("found returns transaction with purchase context", func(t *testing.T) {
		merchant := merchantID(t)
		p := seedProduct(t, ctx, merchant)
		tx := newPurchaseTx(merchant, "ord", "tok", p.ID, "LOOKUP_K")
		require.NoError(t, repo.Create(ctx, tx))

		got, err := repo.GetByPurchaseIdempotencyKey(ctx, merchant, "LOOKUP_K")
		require.NoError(t, err)
		assert.Equal(t, tx.ID, got.ID)
		assert.Equal(t, "LOOKUP_K", got.PurchaseIdempotencyKey)
		require.NotNil(t, got.ProductID)
		assert.Equal(t, p.ID, *got.ProductID)
	})

	t.Run("not found returns ErrNotFound", func(t *testing.T) {
		merchant := merchantID(t)
		_, err := repo.GetByPurchaseIdempotencyKey(ctx, merchant, "DOES_NOT_EXIST")
		assert.True(t, errors.Is(err, transaction.ErrNotFound), "want ErrNotFound, got %v", err)
	})

	t.Run("cross-merchant lookup with same key returns ErrNotFound", func(t *testing.T) {
		merchantA := merchantID(t)
		merchantB := merchantID(t)
		pA := seedProduct(t, ctx, merchantA)
		txA := newPurchaseTx(merchantA, "ord", "tok", pA.ID, "ISO_KEY")
		require.NoError(t, repo.Create(ctx, txA))

		_, err := repo.GetByPurchaseIdempotencyKey(ctx, merchantB, "ISO_KEY")
		assert.True(t, errors.Is(err, transaction.ErrNotFound), "cross-merchant lookup must not leak; got %v", err)
	})
}
