//go:build integration

package transaction_test

import (
	"context"
	"fmt"
	"log/slog"
	"os"
	"sync"
	"testing"
	"time"

	"TestTaskJustPay/pkg/postgres"
	"TestTaskJustPay/pkg/testinfra"
	silvergate "TestTaskJustPay/services/silvergate"
	"TestTaskJustPay/services/silvergate/acquirer"
	"TestTaskJustPay/services/silvergate/domain/transaction"
	txrepo "TestTaskJustPay/services/silvergate/repo"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

var pg *postgres.Postgres

func TestMain(m *testing.M) {
	ctx := context.Background()
	pgContainer, err := testinfra.NewPostgresWithConfig(ctx, testinfra.PostgresConfig{
		DBName:      "silvergate_race_test",
		MigrationFS: silvergate.MigrationFS(),
		Image:       "postgres:17",
	})
	if err != nil {
		panic(fmt.Sprintf("postgres: %v", err))
	}
	pg = pgContainer.Pool
	code := m.Run()
	pgContainer.Cleanup(ctx)
	os.Exit(code)
}

// stubWebhooks tracks async refund completions via channel.
type stubWebhooks struct {
	refundDone chan struct{}
}

func (w *stubWebhooks) SendCaptureResult(context.Context, *transaction.Transaction) error {
	return nil
}

func (w *stubWebhooks) SendRefundResult(_ context.Context, _ *transaction.Transaction, _ *transaction.Refund) error {
	w.refundDone <- struct{}{}
	return nil
}

// TestConcurrentRefund_Overdraft reproduces the lost-update race condition
// in Service.Refund (service.go:130). Three concurrent refunds on a $50
// payment all read refunded_amount=0 and pass validation simultaneously.
//
// Expected: at most 1 refund accepted, total refunded ≤ $50.
// Actual (bug): all 3 accepted, total refunded = $115.
func TestConcurrentRefund_Overdraft(t *testing.T) {
	ctx := context.Background()

	repo := txrepo.NewPgTransactionRepo(pg.Pool)
	acq := acquirer.NewMockAcquirer(1.0, 1.0, 50*time.Millisecond)
	wh := &stubWebhooks{refundDone: make(chan struct{}, 10)}
	txRepoFactory := func(tx postgres.Executor) transaction.Repo {
		return txrepo.NewPgTransactionRepo(tx)
	}
	svc := transaction.NewService(repo, acq, wh, slog.Default(), pg, txRepoFactory)

	// --- Setup: auth + capture a $50 transaction ---

	auth, err := svc.Authorize(ctx, transaction.AuthRequest{
		MerchantID: "merchant_race",
		OrderID:    fmt.Sprintf("race_%d", time.Now().UnixNano()),
		Amount:     5000,
		Currency:   "USD",
		CardToken:  "tok_race",
	})
	require.NoError(t, err)
	require.Equal(t, transaction.StatusAuthorized, auth.Status)

	_, err = svc.Capture(ctx, transaction.CaptureRequest{
		TransactionID:  auth.TransactionID,
		Amount:         5000,
		IdempotencyKey: fmt.Sprintf("cap_%d", time.Now().UnixNano()),
	})
	require.NoError(t, err)

	// Poll until async settle completes
	deadline := time.Now().Add(5 * time.Second)
	for time.Now().Before(deadline) {
		tx, _ := repo.GetByID(ctx, auth.TransactionID)
		if tx != nil && tx.Status == transaction.StatusCaptured {
			break
		}
		time.Sleep(50 * time.Millisecond)
	}
	tx, err := repo.GetByID(ctx, auth.TransactionID)
	require.NoError(t, err)
	require.Equal(t, transaction.StatusCaptured, tx.Status)

	// --- 3 concurrent refunds: $30 + $40 + $45 on a $50 payment ---

	amounts := []int64{3000, 4000, 4500}
	errs := make([]error, len(amounts))

	var wg sync.WaitGroup
	wg.Add(len(amounts))
	for i, amt := range amounts {
		go func(idx int, amount int64) {
			defer wg.Done()
			_, errs[idx] = svc.Refund(ctx, transaction.RefundRequest{
				TransactionID:  auth.TransactionID,
				Amount:         amount,
				IdempotencyKey: fmt.Sprintf("ref_%d_%d", idx, time.Now().UnixNano()),
			})
		}(i, amt)
	}
	wg.Wait()

	// Count how many passed validation
	var accepted int
	for _, e := range errs {
		if e == nil {
			accepted++
		}
	}
	t.Logf("validation: %d/%d refunds accepted", accepted, len(amounts))

	// Wait for async processing of accepted refunds
	for range accepted {
		select {
		case <-wh.refundDone:
		case <-time.After(5 * time.Second):
			t.Fatal("timeout waiting for refund async completion")
		}
	}

	// Query actual total refunded from refund records (source of truth)
	var totalRefunded int64
	err = pg.Pool.QueryRow(ctx,
		"SELECT COALESCE(SUM(amount), 0) FROM refunds WHERE transaction_id = $1 AND status = 'refunded'",
		auth.TransactionID,
	).Scan(&totalRefunded)
	require.NoError(t, err)

	t.Logf("result: accepted=%d, total_refunded=%d, tx_amount=%d", accepted, totalRefunded, tx.Amount)

	// BUG: all 3 pass validation because each reads refunded_amount=0
	assert.Less(t, accepted, len(amounts),
		"not all refunds should pass: $30+$40+$45 exceeds $50")

	// BUG: sum of successful refund records exceeds payment amount
	assert.LessOrEqual(t, totalRefunded, tx.Amount,
		"total refunded (%d) must not exceed payment (%d)", totalRefunded, tx.Amount)
}
